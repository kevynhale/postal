/*
Copyright 2016 Jive Communications All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
)

// NetworkManager defines the interface for how to interact with a Network of addresses.
type NetworkManager interface {
	Pools(filters map[string]string) ([]*api.Pool, error)
	Pool(ID string) (PoolManager, error)
	NewPool(annotations map[string]string, max uint64, poolType api.Pool_Type) (PoolManager, error)
	Binding(net.IP) (*api.Binding, error)
	Bindings(filters map[string]string) ([]*api.Binding, error)
	APINetwork() *api.Network

	// Reservations
	// A reservation is a block of addresses that cannot be used in a pool regaurdless of
	// if it has been claimed or not.
	// This is specifically to address addresses that you don't want to be bound but
	// are not managed directly in postal.

	// Range lists reservations for the networks after applying specified filters
	Reservations(filters map[string]string) ([]*api.Reservation, error)
	// Add creates a new reservation for the network
	AddReservation(cidr string, annotations map[string]string) (*api.Reservation, error)
	// Remove removes an existing reservation from the network
	RemoveReservation(id string) error
	// IsReserved returns true if an existing network reservation matches the specified addr
	IsReserved(ip net.IP) (bool, error)
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

func (nm *etcdNetworkManager) NewPool(annotations map[string]string, max uint64, poolType api.Pool_Type) (PoolManager, error) {
	pool := &api.Pool{
		Annotations:      mergeMap(nm.annotations, annotations),
		MaximumAddresses: max,
		Type:             poolType,
		ID: &api.Pool_PoolID{
			NetworkID: nm.ID,
			ID:        newPoolID(),
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

func (nm *etcdNetworkManager) Bindings(filters map[string]string) ([]*api.Binding, error) {
	pools, err := nm.Pools(nil)
	if err != nil {
		return nil, errors.Wrapf(err, "get pools failed")
	}

	bindings := []*api.Binding{}
	for idx := range pools {
		pm := &etcdPoolManager{
			etcd: nm.etcd,
			pool: pools[idx],
			IPAM: nm.IPAM,
		}
		etcdBindings, err := pm.listBindings(filters)
		if err != nil {
			return nil, errors.Wrapf(err, "get bindings for pool %s failed", pools[idx].ID.ID)
		}
		for _, binding := range etcdBindings {
			bindings = append(bindings, binding.Binding)
		}

	}

	return bindings, nil
}
