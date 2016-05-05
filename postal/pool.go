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

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/twinj/uuid"
)

type PoolManager interface {
	AllocateAddress(annotations map[string]string, requestedAddress net.IP) (*api.Binding, error)
	AllocateMultipleAddresses(annotations map[string]string, addresses uint) (*api.Binding, error)
	ReleaseBinding(*api.Binding) error
}

type etcdPoolManager struct {
	etcd *clientv3.Client
	pool *api.Pool
	IPAM ipam.IPAM
}

type BindError map[string]error

func (err BindError) Error() string {
	errStr := "bind error: "
	for ip, e := range err {
		errStr += fmt.Sprintf("\t%s: %s\n", ip, e.Error())
	}
	return errStr
}

func (pm *etcdPoolManager) AllocateAddress(annotations map[string]string, requestedAddress net.IP) (*api.Binding, error) {
	binding := &api.Binding{
		PoolID:      pm.pool.ID,
		ID:          uuid.NewV4().String(),
		Annotations: annotations,
	}
	if requestedAddress.IsUnspecified() {
		ipnet, err := pm.IPAM.Allocate(1)
		if err != nil {
			return nil, err
		}

		binding.Addresses = []string{
			ipnet[0].IP.String(),
		}

	} else {
		err := pm.IPAM.Claim(requestedAddress)
		if err != nil {
			return nil, err
		}

		binding.Addresses = []string{
			requestedAddress.String(),
		}
	}

	return binding, nil
}

func (pm *etcdPoolManager) AllocateMultipleAddresses(annotations map[string]string, addresses uint) (*api.Binding, error) {
	binding := &api.Binding{
		PoolID:      pm.pool.ID,
		ID:          uuid.NewV4().String(),
		Annotations: annotations,
		Addresses:   []string{},
	}

	addrs, err := pm.IPAM.Allocate(addresses)
	if err != nil {
		return nil, err
	}

	for _, ipnet := range addrs {
		binding.Addresses = append(binding.Addresses, ipnet.IP.String())
	}
	return binding, nil
}

func (pm *etcdPoolManager) ReleaseBinding(binding *api.Binding) error {
	errs := BindError{}
	for _, addr := range binding.Addresses {
		ip := net.ParseIP(addr)
		err := pm.IPAM.Release(ip)
		if err != nil {
			errs[ip.String()] = err
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
