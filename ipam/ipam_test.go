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

package ipam

import (
	"net"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
)

func TestIPAMOutOfRangeClaim(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("10.10.0.0/24", cli)
	assert.NoError(err)

	assert.Error(i.Claim(net.ParseIP("10.20.0.10")))
}

func TestIPAMFragmentedClaim(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("10.10.0.0/22", cli)
	assert.NoError(err)

	_, err = i.Allocate(30)
	assert.NoError(err)

	assert.NoError(i.Claim(net.ParseIP("10.10.2.10")))

	_, err = i.Allocate(250)
	assert.NoError(err)

	assert.NoError(i.Claim(net.ParseIP("10.10.3.10")))
}

func TestIPAMOutOfRangeClaimV6(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("2001:db8::/112", cli)
	assert.NoError(err)

	assert.Error(i.Claim(net.ParseIP("2001:db8:1::/112")))
}

func TestIPAMFragmentedClaimV6(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("2001:db8::/110", cli)
	assert.NoError(err)

	_, err = i.Allocate(30)
	assert.NoError(err)

	assert.NoError(i.Claim(net.ParseIP("2001:db8::1:0001")))

	_, err = i.Allocate(65536)
	assert.NoError(err)

	assert.NoError(i.Claim(net.ParseIP("2001:db8::3:0001")))
}

func TestIPAM_IT(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("10.10.0.0/16", cli)
	assert.NoError(err)

	// Concurrent goroutines
	txn := 25
	// Number of transactions per goroutine
	batches := 20
	// Number of addresses to allocate each transaction
	count := 20

	// addrChan will collect the allocations
	addrChan := make(chan []net.IP)
	for idx := 0; idx < txn; idx++ {
		go func() {
			for j := 0; j < batches; j++ {
				addrs, err := i.Allocate(uint(count))
				if err != nil {
					t.Error(err)
				}

				addrChan <- addrs
			}
		}()
	}

	// set of addresses to use for uniqueness checks
	finalAddrs := map[string]struct{}{}

	for idx := 0; idx < txn*batches; idx++ {
		select {
		case addrs := <-addrChan:
			// Assert each allocation matches the specified size
			assert.Equal(count, len(addrs))
			// Assert that every address is unique
			for _, addr := range addrs {
				if _, ok := finalAddrs[addr.String()]; ok {
					t.Fatal("Address is a duplicate: ", addr.String())
				} else {
					finalAddrs[addr.String()] = struct{}{}
				}
			}
		}
	}

	// release 50 addresses
	released := map[string]struct{}{}
	for addr := range finalAddrs {
		if len(released) > count {
			break
		}
		err := i.Release(net.ParseIP(addr))
		if err != nil {
			t.Fatalf("Error releasing address: %v", err)
		}
		released[addr] = struct{}{}
	}

	// Allocate an address and assert that it was from the released addresses.
	// TODO: make this not flaky
	// addrs, _ := i.Allocate(1)
	// _, ok := released[addrs[0].String()]
	// assert.True(ok)
}
