package postal

import (
	"encoding/json"
	"net"
	"strings"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/twinj/uuid"
)

type NetworkManager interface {
	Pools() ([]string, error)
	Pool(ID string) (PoolManager, error)
	NewPool(annotations map[string]string, min, max int, poolType api.Pool_Type) (PoolManager, error)
	Bindings(net.IP) ([]*api.Binding, error)
}

type etcdNetworkManager struct {
	ID          string
	cidr        string
	annotations map[string]string

	IPAM ipam.IPAM
	etcd *clientv3.Client
}

func (nm *etcdNetworkManager) Pools() ([]string, error) {
	resp, err := nm.etcd.KV.Get(context.Background(), networkPoolsKey(nm.ID), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	poolIDs := []string{}
	for idx := range resp.Kvs {
		poolIDs = append(poolIDs, strings.Split(string(resp.Kvs[idx].Key), "/")[7])
	}
	return poolIDs, nil
}

func (nm *etcdNetworkManager) Pool(ID string) (PoolManager, error) {
	resp, err := nm.etcd.Get(context.TODO(), poolMetaKey(nm.ID, ID))
	if err != nil {
		return nil, err
	}

	pool := &api.Pool{}
	err = json.Unmarshal(resp.Kvs[0].Value, pool)
	if err != nil {
		return nil, err
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

	poolBytes, err := pool.Marshal()
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

func (nm *etcdNetworkManager) Bindings(addr net.IP) ([]*api.Binding, error) {
	resp, err := nm.etcd.KV.Get(context.Background(), bindingListAddrKey(nm.ID, addr), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	bindings := []*api.Binding{}
	for idx := range resp.Kvs {
		binding := &api.Binding{}
		json.Unmarshal(resp.Kvs[idx].Value, binding)
		bindings = append(bindings, binding)
	}
	return bindings, nil
}
