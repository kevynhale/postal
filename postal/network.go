package postal

import (
	"encoding/json"
	"net"
	"regexp"
	"strings"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/pkg/errors"
	"github.com/twinj/uuid"
)

// NetworkManager defines the interface for how to interact with a Network of addresses.
type NetworkManager interface {
	Pools(filters map[string]string) ([]*api.Pool, error)
	Pool(ID string) (PoolManager, error)
	NewPool(annotations map[string]string, min, max int, poolType api.Pool_Type) (PoolManager, error)
	Binding(net.IP) (*api.Binding, error)
	APINetwork() *api.Network
}

type etcdNetworkManager struct {
	ID          string
	cidr        string
	annotations map[string]string

	IPAM ipam.IPAM
	etcd *clientv3.Client
}

func (nm *etcdNetworkManager) APINetwork() *api.Network {
	return &api.Network{
		ID:          nm.ID,
		Annotations: nm.annotations,
		Cidr:        nm.cidr,
	}
}

func (nm *etcdNetworkManager) Pools(filters map[string]string) ([]*api.Pool, error) {
	resp, err := nm.etcd.KV.Get(context.Background(), networkPoolsKey(nm.ID), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	noFilter := filters == nil || len(filters) == 0

	pools := []*api.Pool{}
	for idx := range resp.Kvs {
		pool := &api.Pool{}
		err = json.Unmarshal(resp.Kvs[idx].Value, pool)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal pool")
		}

		if noFilter {
			pools = append(pools, pool)
		} else {
			var matched bool
			for field, filter := range filters {
				switch field {
				case "_id":
					matched, err = regexp.MatchString(filter, pool.ID.ID)
				case "_network":
					matched, err = regexp.MatchString(filter, pool.ID.NetworkID)
				case "_type":
					matched, err = regexp.MatchString(strings.ToLower(filter), strings.ToLower(pool.Type.String()))
				default:
					if val, ok := pool.Annotations[field]; ok {
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
				pools = append(pools, pool)
			}
		}
	}

	return pools, nil
}

func (nm *etcdNetworkManager) Pool(ID string) (PoolManager, error) {
	resp, err := nm.etcd.Get(context.TODO(), poolMetaKey(nm.ID, ID))
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) != 1 {
		return nil, errors.New("pool not found")
	}

	pool := &api.Pool{}
	err = json.Unmarshal(resp.Kvs[0].Value, pool)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	return &etcdPoolManager{
		etcd: nm.etcd,
		pool: pool,
		IPAM: nm.IPAM,
	}, nil
}

func (nm *etcdNetworkManager) NewPool(annotations map[string]string, min, max int, poolType api.Pool_Type) (PoolManager, error) {
	pool := &api.Pool{
		Annotations:      annotations,
		MinimumAddresses: int32(min),
		MaximumAddresses: int32(max),
		Type:             poolType,
		ID: &api.Pool_PoolID{
			NetworkID: nm.ID,
			ID:        uuid.NewV4().String(),
		},
	}

	poolBytes, err := json.Marshal(pool)
	if err != nil {
		return nil, err
	}

	_, err = nm.etcd.KV.Put(
		context.TODO(),
		poolMetaKey(nm.ID, pool.ID.ID),
		string(poolBytes),
	)

	if err != nil {
		return nil, err
	}

	return &etcdPoolManager{
		etcd: nm.etcd,
		pool: pool,
		IPAM: nm.IPAM,
	}, nil
}

func (nm *etcdNetworkManager) Binding(addr net.IP) (*api.Binding, error) {
	binding, err := nm.getBindingForAddr(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "get binding for address %s failed", addr.String())
	}

	return binding.Binding, nil
}
