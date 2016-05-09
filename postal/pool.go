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
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/twinj/uuid"
)

type PoolManager interface {
	AllocateAddress(annotations map[string]string, requestedAddress net.IP) (*api.Binding, error)
	AllocateMultipleAddresses(annotations map[string]string, addresses uint) (*api.Binding, error)
	ReleaseBinding(*api.Binding) error
	LookupBinding(ID string) (*api.Binding, error)
	GetID() string
}

type etcdPoolManager struct {
	etcd *clientv3.Client
	pool *api.Pool
	IPAM ipam.IPAM
}

func (pm *etcdPoolManager) GetID() string {
	return pm.pool.ID.ID
}

// ReleasedBindingTTL defines the period of time in seconds for which a
// released binding is kept for informational purposes.
const ReleasedBindingTTL = 60 * 60 * 6

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
		BindTime:    time.Now().UTC().UnixNano(),
		ReleaseTime: -1,
	}
	if requestedAddress == nil || requestedAddress.IsUnspecified() {
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

	err := pm.writeBinding(binding)
	if err != nil {
		for idx := range binding.Addresses {
			pm.IPAM.Release(net.ParseIP(binding.Addresses[idx]))
		}
		return nil, err
	}

	return binding, nil
}

func (pm *etcdPoolManager) AllocateMultipleAddresses(annotations map[string]string, addresses uint) (*api.Binding, error) {
	binding := &api.Binding{
		PoolID:      pm.pool.ID,
		ID:          uuid.NewV4().String(),
		Annotations: annotations,
		Addresses:   []string{},
		BindTime:    time.Now().UTC().UnixNano(),
		ReleaseTime: -1,
	}

	addrs, err := pm.IPAM.Allocate(addresses)
	if err != nil {
		return nil, err
	}

	for _, ipnet := range addrs {
		binding.Addresses = append(binding.Addresses, ipnet.IP.String())
	}

	err = pm.writeBinding(binding)
	if err != nil {
		for idx := range binding.Addresses {
			pm.IPAM.Release(net.ParseIP(binding.Addresses[idx]))
		}
		return nil, err
	}

	return binding, nil
}

func (pm *etcdPoolManager) ReleaseBinding(binding *api.Binding) error {
	binding.ReleaseTime = time.Now().UTC().UnixNano()
	err := pm.writeBinding(binding)
	if err != nil {
		return err
	}

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

func (pm *etcdPoolManager) LookupBinding(ID string) (*api.Binding, error) {
	resp, err := pm.etcd.KV.Get(
		context.TODO(),
		bindingIDKey(pm.pool.ID.NetworkID, pm.pool.ID.ID, ID),
	)

	if err != nil {
		return nil, err
	}

	if resp.Kvs[0] == nil {
		return nil, fmt.Errorf("Binding not found: %s", ID)
	}

	binding := &api.Binding{}
	err = json.Unmarshal(resp.Kvs[0].Value, binding)
	if err != nil {
		return nil, err
	}

	return binding, nil
}

func (pm *etcdPoolManager) writeBinding(binding *api.Binding) error {
	data, err := binding.Marshal()
	if err != nil {
		return err
	}

	putOpOptions := []clientv3.OpOption{}
	if binding.ReleaseTime > 0 {
		resp, err := pm.etcd.Lease.Grant(context.TODO(), ReleasedBindingTTL)
		if err != nil {
			return err
		}

		putOpOptions = append(putOpOptions, clientv3.WithLease(resp.ID))
	}

	ops := []clientv3.Op{}
	for idx := range binding.Addresses {
		ops = append(ops, clientv3.OpPut(
			bindingAddrKey(binding.PoolID.NetworkID, binding.ID, net.ParseIP(binding.Addresses[idx])),
			string(data), putOpOptions...))
	}

	ops = append(ops, clientv3.OpPut(bindingIDKey(
		pm.pool.ID.NetworkID,
		pm.pool.ID.ID,
		binding.ID,
	), string(data), putOpOptions...))

	_, err = pm.etcd.KV.Txn(context.TODO()).Then(ops...).Commit()

	if err != nil {
		return err
	}

	return nil
}
