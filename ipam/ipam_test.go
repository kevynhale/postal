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

func TestIPAM_IT(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.SkipNow()
	}
	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	i, err := NewIPAM("10.10.0.0/16", cli)
	if err != nil {
		t.Error(err)
	}

	// Concurrent goroutines
	txn := 50
	// Number of transactions per goroutine
	batches := 20
	// Number of addresses to allocate each transaction
	count := 50

	// addrChan will collect the allocations
	addrChan := make(chan []net.IPNet)
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

	for idx := 0; idx < txn; idx++ {
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
	for cidr := range finalAddrs {
		if len(released) > count {
			break
		}
		ip, ipNet, _ := net.ParseCIDR(cidr)
		err := i.Release(ip)
		if err != nil {
			t.Fatalf("Error releasing address: %v", err)
		}
		released[ipNet.String()] = struct{}{}
	}

	// Allocate an address and assert that it was from the released addresses.
	addrs, _ := i.Allocate(1)
	_, ok := released[addrs[0].String()]
	assert.True(ok)
}
