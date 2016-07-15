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
	"github.com/coreos/pkg/capnslog"
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
			MaximumAddresses: 5,
			Type:             api.Pool_FIXED,
		},
		IPAM: IPAM,
	}
}

func TestConcurrentBind(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	nm, err := (&Config{}).WithEtcdClient(cli).NewNetwork(nil, "10.0.0.0/24")
	assert.NoError(err)

	pool, err := nm.NewPool(nil, 50, api.Pool_FIXED)
	assert.NoError(err)

	for i := 0; i < 10; i++ {
		_, err := pool.Allocate(nil)
		assert.NoError(err)
	}

	addrChan := make(chan string)

	for i := 0; i < 10; i++ {
		go func() {
			b, err := pool.BindAny(map[string]string{})
			assert.NoError(err)
			if err == nil {
				addrChan <- b.Address
			}
		}()
	}

	addresses := map[string]struct{}{}
	for i := 0; i < 10; i++ {
		addr := <-addrChan
		if _, ok := addresses[addr]; ok {
			t.Error("duplicate bound address: " + addr)
		} else {
			addresses[addr] = struct{}{}
		}
	}
}

func TestAllocate(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	nm, err := (&Config{}).WithEtcdClient(cli).NewNetwork(nil, "10.0.0.0/24")
	assert.NoError(err)

	pool, err := nm.NewPool(nil, 5, api.Pool_FIXED)
	assert.NoError(err)

	binding, err := pool.Allocate(nil)
	assert.NoError(err)
	assert.NotNil(binding)

	assert.Equal(pool.APIPool().ID.NetworkID, binding.PoolID.NetworkID)
	assert.Equal(pool.APIPool().ID.ID, binding.PoolID.ID)
	assert.Equal("10.0.0.1", binding.Address)

	binding2, err := pool.Allocate(net.ParseIP("10.0.0.3"))
	assert.NoError(err)
	assert.NotNil(binding2)
	assert.Equal("10.0.0.3", binding2.Address)

	binding3, err := pool.Allocate(net.ParseIP("10.0.0.3"))
	assert.Error(err)
	assert.Nil(binding3)
}

func TestReleaseHard(t *testing.T) {
	capnslog.SetGlobalLogLevel(capnslog.DEBUG)
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	j := &Janitor{etcd: cli}
	go j.Run()

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	nm, err := (&Config{}).WithEtcdClient(cli).NewNetwork(nil, "10.0.0.0/24")
	assert.NoError(err)

	pool, err := nm.NewPool(nil, 5, api.Pool_FIXED)
	assert.NoError(err)

	binding, err := pool.Allocate(net.ParseIP("10.0.0.1"))
	assert.NoError(err)
	assert.NotNil(binding)

	assert.Equal(pool.APIPool().ID.NetworkID, binding.PoolID.NetworkID)
	assert.Equal(pool.APIPool().ID.ID, binding.PoolID.ID)
	assert.Equal("10.0.0.1", binding.Address)

	assert.NoError(pool.Release(binding, true))
	assert.Error(pool.Release(binding, true))

	//Give time for the binding to clear
	time.Sleep(1 * time.Second)
	binding, err = pool.Allocate(net.ParseIP("10.0.0.1"))
	assert.NoError(err)
	assert.NotNil(binding)

	assert.Equal(pool.APIPool().ID.NetworkID, binding.PoolID.NetworkID)
	assert.Equal(pool.APIPool().ID.ID, binding.PoolID.ID)
	assert.Equal("10.0.0.1", binding.Address)
}

func TestSetMaxSize(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	pool := mkPool(cli, "10.0.0.0/24")
	for i := uint64(0); i < pool.MaxSize(); i++ {
		_, err = pool.Allocate(nil)
		assert.NoError(err)
	}

	err = pool.SetMaxSize(2)
	assert.Error(err)

	err = pool.SetMaxSize(6)
	assert.NoError(err)

	_, err = pool.Allocate(nil)
	assert.NoError(err)

	_, err = pool.Allocate(nil)
	assert.Error(err)
}
