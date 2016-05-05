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

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
)

func TestIPAM(t *testing.T) {
	t.SkipNow()
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2378"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.SkipNow()
	}
	defer cli.Close()

	//i, err := NewIPAM("2001:db8::/112", cli)
	i, err := NewIPAM("10.117.0.0/16", cli)
	if err != nil {
		t.Error(err)
	}

	txn := 50
	batches := 20
	count := 50

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

	finalAddrs := map[string]struct{}{}

	for idx := 0; idx < txn; idx++ {
		select {
		case addrs := <-addrChan:
			assert.Equal(count, len(addrs))
			for _, addr := range addrs {
				if _, ok := finalAddrs[addr.String()]; ok {
					t.Fatal("Address is a duplicate: ", addr.String())
				} else {
					finalAddrs[addr.String()] = struct{}{}
				}
				//fmt.Println(addr)
			}
		}
	}

	released := 0
	for cidr, _ := range finalAddrs {
		if released > count {
			break
		}
		ip, _, _ := net.ParseCIDR(cidr)
		err := i.Release(ip)
		if err != nil {
			t.Fatalf("Error releasing address: %v", err)
		}
		released++
	}

	addrs, _ := i.Allocate(1)
	_, ok := finalAddrs[addrs[0].String()]
	assert.True(ok)
}
