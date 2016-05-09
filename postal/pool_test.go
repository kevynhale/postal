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
	"net"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/stretchr/testify/assert"
)

func mkPool(etcd *clientv3.Client, cidr string) *etcdPoolManager {
	IPAM, _ := ipam.NewIPAM(cidr, etcd)
	return &etcdPoolManager{
		etcd: etcd,
		pool: &api.Pool{
			ID: &api.Pool_PoolID{
				NetworkID: "network1",
				ID:        "pool1",
			},
			Annotations:      map[string]string{"foo": "bar"},
			MinimumAddresses: 2,
			MaximumAddresses: 5,
			Type:             api.Pool_FIXED,
		},
		IPAM: IPAM,
	}
}

func TestAllocateAddress(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	pool := mkPool(cli, "10.0.0.0/24")
	binding, err := pool.AllocateAddress(map[string]string{"abc": "123"}, nil)
	assert.NoError(err)
	assert.NotNil(binding)

	assert.Equal(pool.pool.ID.NetworkID, binding.PoolID.NetworkID)
	assert.Equal(pool.pool.ID.ID, binding.PoolID.ID)
	assert.Equal(1, len(binding.Addresses))
	assert.Equal("10.0.0.1", binding.Addresses[0])
	assert.Equal("123", binding.Annotations["abc"])

	binding2, err := pool.AllocateAddress(map[string]string{}, net.ParseIP("10.0.0.3"))
	assert.NoError(err)
	assert.NotNil(binding2)
	assert.Equal(1, len(binding2.Addresses))
	assert.Equal("10.0.0.3", binding2.Addresses[0])
}

func TestAllocateMultipleAddresses(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	pool := mkPool(cli, "10.0.0.0/24")
	binding, err := pool.AllocateMultipleAddresses(map[string]string{}, uint(10))
	assert.NoError(err)
	assert.NotNil(binding)

	assert.Equal(pool.pool.ID.NetworkID, binding.PoolID.NetworkID)
	assert.Equal(pool.pool.ID.ID, binding.PoolID.ID)
	assert.Equal(10, len(binding.Addresses))
	for idx := range binding.Addresses {
		assert.Equal(byte(idx+1), net.ParseIP(binding.Addresses[idx]).To4()[3])
	}
}
