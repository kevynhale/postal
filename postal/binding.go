package postal

import (
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	// DefaultReleasedBindingTTL defines the period of time in seconds for which a
	// released binding is kept for informational purposes.
	DefaultReleasedBindingTTL = 60 * 60 * 6
	// NoTTL indicates that a ttl should not be set when writing a binding.
	NoTTL = 0
	// HardRelease indicates that the binding should immediately expire.
	HardRelease = 1
)

type etcdBinding struct {
	*api.Binding

	version int64
}

func (b *etcdBinding) isBound() bool {
	return b.BindTime > b.ReleaseTime
}

func (b *etcdBinding) annotate(key, value string) {
	b.Annotations[key] = value
}

func (b *etcdBinding) etcdConditions() []clientv3.Cmp {
	return []clientv3.Cmp{
		clientv3.Compare(clientv3.Version(bindingIDKey(b.PoolID.NetworkID, b.PoolID.ID, b.ID)), "=", b.version),
	}
}

func filterBoundBindings(bindings []*etcdBinding) []*etcdBinding {
	filtered := []*etcdBinding{}
	for idx := range bindings {
		if !bindings[idx].isBound() {
			filtered = append(filtered, bindings[idx])
		}
	}
	return filtered
}

func newBinding(b *api.Binding) *etcdBinding {
	binding := &etcdBinding{
		b,
		0,
	}
	binding.AllocateTime = time.Now().UTC().UnixNano()
	return binding
}

func (pm *etcdPoolManager) allocateBinding(binding *etcdBinding, addr net.IP) error {
	if addr == nil || addr.IsUnspecified() {
		return errors.New("must specify an address")
	}

	resp, err := pm.etcd.Get(context.Background(), bindingAddrKey(pm.pool.ID.NetworkID, addr))
	if err != nil {
		return err
	}

	if len(resp.Kvs) != 0 {
		return errors.New("address already allocated")
	}
	binding.AllocateTime = time.Now().UTC().UnixNano()
	binding.Address = addr.String()

	return pm.writeBinding(binding, NoTTL)
}

func (pm *etcdPoolManager) bindBinding(binding *etcdBinding, addr net.IP) error {
	timestamp := time.Now().UTC().UnixNano()
	if binding.AllocateTime == 0 {
		binding.AllocateTime = timestamp
	}
	binding.BindTime = timestamp
	binding.Address = addr.String()
	return pm.writeBinding(binding, NoTTL)
}

func (pm *etcdPoolManager) rebindBinding(binding *etcdBinding, annotations map[string]string) error {
	binding.Binding.Annotations = annotations
	binding.Binding.BindTime = time.Now().UTC().UnixNano()
	return pm.writeBinding(binding, NoTTL)
}

func (pm *etcdPoolManager) releaseBinding(binding *etcdBinding, ttl int64) error {
	binding.ReleaseTime = time.Now().UTC().UnixNano()
	return pm.writeBinding(binding, ttl)
}

func (pm *etcdPoolManager) writeBinding(binding *etcdBinding, ttl int64) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return errors.Wrap(err, "marshalling binding failed")
	}

	putOpOptions := []clientv3.OpOption{}
	if ttl > NoTTL {
		resp, err := pm.etcd.Lease.Grant(context.TODO(), ttl)
		if err != nil {
			return errors.Wrap(err, "creating lease failed")
		}

		putOpOptions = append(putOpOptions, clientv3.WithLease(resp.ID))
	}

	var ops []clientv3.Op
	if ttl == HardRelease {
		ops = []clientv3.Op{
			clientv3.OpDelete(bindingAddrKey(binding.PoolID.NetworkID, net.ParseIP(binding.Address))),
			clientv3.OpDelete(bindingIDKey(pm.pool.ID.NetworkID, pm.pool.ID.ID, binding.ID)),
		}
	} else {
		ops = []clientv3.Op{
			clientv3.OpPut(
				bindingAddrKey(binding.PoolID.NetworkID, net.ParseIP(binding.Address)),
				bindingIDKey(binding.PoolID.NetworkID, binding.PoolID.ID, binding.ID), putOpOptions...),
			clientv3.OpPut(bindingIDKey(
				pm.pool.ID.NetworkID,
				pm.pool.ID.ID,
				binding.ID,
			), string(data), putOpOptions...),
		}
	}

	res, err := pm.etcd.KV.Txn(context.TODO()).If(binding.etcdConditions()...).Then(ops...).Commit()

	if err != nil {
		return errors.Wrap(err, "etcd transaction error")
	}

	if !res.Succeeded {
		return errors.New("etcd transaction failed")
	}

	return nil
}

func (pm *etcdPoolManager) listBindings(filters map[string]string) ([]*etcdBinding, error) {
	resp, err := pm.etcd.KV.Get(context.Background(), bindingListKey(pm.pool.ID.NetworkID, pm.pool.ID.ID), clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "etcd kv range failed")
	}

	noFilter := filters == nil || len(filters) == 0

	bindings := []*etcdBinding{}
	for idx := range resp.Kvs {
		binding := &api.Binding{}
		json.Unmarshal(resp.Kvs[idx].Value, binding)

		if noFilter {
			bindings = append(bindings, &etcdBinding{binding, resp.Kvs[idx].Version})
		} else {
			var matched bool
			for field, filter := range filters {
				switch field {
				case "_id":
					matched, err = regexp.MatchString(filter, binding.ID)
				case "_pool":
					matched, err = regexp.MatchString(filter, binding.PoolID.ID)
				case "_network":
					matched, err = regexp.MatchString(filter, binding.PoolID.NetworkID)
				case "_address":
					matched, err = regexp.MatchString(filter, binding.Address)
				default:
					if val, ok := binding.Annotations[field]; ok {
						matched, err = regexp.MatchString(filter, val)
					} else {
						break
					}
				}
				if err != nil {
					return nil, errors.Wrapf(err, "failed to compile filter '%s'", filter)
				}

				if !matched {
					break
				}
			}

			if matched {
				bindings = append(bindings, &etcdBinding{binding, resp.Kvs[idx].Version})
			}
		}

	}
	return bindings, nil
}

func (pm *etcdPoolManager) getBinding(ID string) (*etcdBinding, error) {
	resp, err := pm.etcd.KV.Get(context.Background(), bindingIDKey(pm.pool.ID.NetworkID, pm.pool.ID.ID, ID))
	if err != nil {
		return nil, errors.Wrap(err, "etcd kv get failed")
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.Errorf("failed to get binding for ID (%s)", ID)
	}

	binding := &api.Binding{}
	json.Unmarshal(resp.Kvs[0].Value, binding)
	return &etcdBinding{binding, resp.Kvs[0].Version}, nil
}

func (pm *etcdPoolManager) getBindingForAddr(addr net.IP) (*etcdBinding, error) {
	resp, err := pm.etcd.KV.Get(context.Background(), bindingAddrKey(pm.pool.ID.NetworkID, addr))
	if err != nil {
		return nil, errors.Wrap(err, "etcd kv get failed")
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.Errorf("failed to get binding for addr (%s)", addr.String())
	}

	bindingKey := string(resp.Kvs[0].Value)
	return pm.getBinding(strings.Split(bindingKey, "/")[7])
}

func (nm *etcdNetworkManager) getBindingForAddr(addr net.IP) (*etcdBinding, error) {
	resp, err := nm.etcd.KV.Get(context.Background(), bindingAddrKey(nm.ID, addr))
	if err != nil {
		return nil, errors.Wrap(err, "etcd kv get failed")
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.Errorf("failed to get binding for addr (%s)", addr.String())
	}

	bindingKey := string(resp.Kvs[0].Value)
	resp, err = nm.etcd.KV.Get(context.Background(), bindingKey)
	if err != nil {
		return nil, errors.Wrap(err, "etcd kv get failed")
	}

	binding := &api.Binding{}
	json.Unmarshal(resp.Kvs[0].Value, binding)
	return &etcdBinding{binding, resp.Kvs[0].Version}, nil
}
