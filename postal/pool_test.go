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
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
)

func Test(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Error(err)
	}

	config := (&Config{}).WithEtcdClient(cli)

	network, err := config.NewNetwork(map[string]string{}, "10.117.0.0/16")
	if err != nil {
		t.Error(err)
	}

	pm1, err := network.NewPool(map[string]string{}, 2, 5, api.Pool_DYNAMIC)
	if err != nil {
		t.Error(err)
	}
	pm1.AllocateAddress(map[string]string{}, nil)

	pm2, err := network.NewPool(map[string]string{}, 2, 5, api.Pool_DYNAMIC)
	if err != nil {
		t.Error(err)
	}

	pm3, err := network.NewPool(map[string]string{}, 2, 5, api.Pool_DYNAMIC)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(pm1.GetID())
	fmt.Println(pm2.GetID())
	fmt.Println(pm3.GetID())

	poolIDs, err := network.Pools()
	if err != nil {
		t.Error(err)
	}

	fmt.Println(poolIDs)

	networkIDs, err := config.Networks()
	if err != nil {
		t.Error(err)
	}

	fmt.Println(networkIDs)
}
