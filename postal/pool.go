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
	"fmt"
	"net"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/pkg/errors"
	"github.com/twinj/uuid"
)

// PoolManager defines the interface for how to interact with a specific pool.
// A pool can be of type DYNAMIC or FIXED.
//
// DYNAMIC pools allow for Bind calls to automatically allocate new addresses if
// the max address count has not been met. Released addresses in a DYNAMIC pool will
// return to the parent network block if they are not rebound within the ttl period.
//
// FIXED pools can only bind addresses from pre allocated bindings. If there are no
// bindings available and the max address count has not been met, the Bind call will
// still fail and return an error.
// Releasing an address in a FIXED pool does not place a ttl on it and it will never be
// released back to the parent network block on its own.
type PoolManager interface {
	// Allocate places an address into the pool to be bound in a subsequent Bind call.
	Allocate(requestedAddress net.IP) (*api.Binding, error)
	// Bind reserves an address such that no other Bind call can claim it.
	// If the pool does not have enough addresses for the request and has not met it's maximum,
	// it will attempt to allocate an additional address for the parent network block.
	Bind(annotations map[string]string, requestedAddress net.IP) (*api.Binding, error)
	// Release will place the address back into a state where it can be bound again within the pool.
	// If the pool is a DYNAMIC type, it will place a TTL on the binding, such that when it expires it
	// is released back into the parent network block.
	//
	// The hard flag, if true, indicates to do a hard release which removed the address from
	// the pool back to the parent network block
	Release(binding *api.Binding, hard bool) error
	// Binding returns the api.Binding for the given ID.
	Binding(ID string) (*api.Binding, error)
	// ID returns the pool's ID
	ID() string
	// CurrentSize will enumerate the existing bindings for a pool and return the cardinatlity.
	CurrentSize() int
	// MaxSize indicates what the maximum number of addresses a pool may hold.
	// A MaxSize of 0, disables this check and allows for a unbounded pool
	MaxSize() int
	// Type will be one of api.Pool_FIXED or api.Pool_DYNAMIC
	Type() api.Pool_Type
}

type etcdPoolManager struct {
	etcd *clientv3.Client
	pool *api.Pool
	IPAM ipam.IPAM
}

func (pm *etcdPoolManager) ID() string {
	return pm.pool.ID.ID
}

func (pm *etcdPoolManager) Type() api.Pool_Type {
	return pm.pool.Type
}

func (pm *etcdPoolManager) Allocate(requestedAddress net.IP) (*api.Binding, error) {
	if pm.CurrentSize() >= pm.MaxSize() {
		return nil, errors.New("allocate failed: maximum addresses reached")
	}
	binding := newBinding(&api.Binding{
		PoolID: pm.pool.ID,
		ID:     uuid.NewV4().String(),
	})

	err := pm.allocateBinding(binding, requestedAddress)
	if err != nil {
		return nil, errors.Wrap(err, "binding allocation failed")
	}

	return binding.Binding, nil
}

func (pm *etcdPoolManager) Bind(annotations map[string]string, requestedAddress net.IP) (*api.Binding, error) {
	binding := newBinding(&api.Binding{
		PoolID:      pm.pool.ID,
		ID:          uuid.NewV4().String(),
		Annotations: annotations,
	})

	if requestedAddress == nil || requestedAddress.IsUnspecified() {
		existingBindings, err := pm.listBindings()
		if err != nil {
			return nil, errors.Wrap(err, "list bindings failed")
		}

		filteredBindings := filterBoundBindings(existingBindings)

		// First, check existing unbound bindings and reuse if any exists
		for idx := range filteredBindings {
			err = pm.rebindBinding(filteredBindings[idx], annotations)
			if err == nil {
				return binding.Binding, nil
			}
		}

		if pm.pool.Type == api.Pool_FIXED {
			return nil, errors.New("bind failed: all allocated addresses in use")
		}

		// No existing binding could be used, so a new address is allocated
		if pm.CurrentSize() >= pm.MaxSize() {
			return nil, errors.New("allocate failed: maximum addresses reached")
		}
		ip, err := pm.IPAM.Allocate(1)
		if err != nil {
			return nil, errors.Wrap(err, "allocating address from ipam failed")
		}

		err = pm.bindBinding(binding, ip[0])
		if err != nil {
			return nil, errors.Wrap(err, "binding address failed")
		}

	} else {
		// Check existing bindings for requested address
		addrBinding, err := pm.getBindingForAddr(requestedAddress)
		if addrBinding != nil && !addrBinding.isBound() {
			err = pm.rebindBinding(addrBinding, annotations)
			if err == nil {
				return addrBinding.Binding, nil
			}
			return nil, fmt.Errorf("address already bound")
		}

		if pm.pool.Type == api.Pool_FIXED {
			return nil, errors.New("bind failed: all allocated addresses in use")
		}

		if pm.CurrentSize() >= pm.MaxSize() {
			return nil, errors.New("allocate failed: maximum addresses reached")
		}
		err = pm.IPAM.Claim(requestedAddress)
		if err != nil {
			return nil, errors.Wrap(err, "address claim failed")
		}

		err = pm.bindBinding(binding, requestedAddress)
		if err != nil {
			return nil, errors.Wrap(err, "binding address failed")
		}
	}

	return binding.Binding, nil
}

func (pm *etcdPoolManager) Release(b *api.Binding, hard bool) error {
	binding, err := pm.getBinding(b.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get binding")
	}

	if hard {
		err = pm.releaseBinding(binding, HardRelease)
		if err != nil {
			return errors.Wrap(err, "failed to hard release binding")
		}
	}

	switch pm.pool.Type {
	case api.Pool_DYNAMIC:
		err = pm.releaseBinding(binding, DefaultReleasedBindingTTL)
		if err != nil {
			return errors.Wrap(err, "failed to release binding")
		}

	case api.Pool_FIXED:
		err = pm.releaseBinding(binding, 0)
		if err != nil {
			return errors.Wrap(err, "failed to release binding")
		}
	}

	return nil
}

func (pm *etcdPoolManager) Binding(ID string) (*api.Binding, error) {
	binding, err := pm.getBinding(ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get binding")
	}

	return binding.Binding, nil
}

func (pm *etcdPoolManager) CurrentSize() int {
	resp, err := pm.etcd.KV.Get(context.Background(), bindingListKey(pm.pool.ID.NetworkID, pm.pool.ID.ID), clientv3.WithPrefix())
	if err != nil {
		return 0
	}

	return len(resp.Kvs)
}

func (pm *etcdPoolManager) MaxSize() int {
	return int(pm.pool.MaximumAddresses)
}
