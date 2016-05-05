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
	"path"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/twinj/uuid"
	"golang.org/x/net/context"
)

// PostalEtcdKeyPrefix defines the prefix used for all postal registry keys stored in etcd
const PostalEtcdKeyPrefix = "/postal/registry/v1/"

type Config struct {
	EtcdEndpoints []string

	etcd *clientv3.Client
}

func (config *Config) WithEtcdClient(etcd *clientv3.Client) *Config {
	config.EtcdEndpoints = etcd.Endpoints()
	config.etcd = etcd
	return config
}

func (config *Config) NewPool(pool *api.Pool, IPAM ipam.IPAM) (PoolManager, error) {
	pool.ID.ID = uuid.NewV4().String()

	poolBytes, err := pool.Marshal()
	if err != nil {
		return nil, err
	}

	_, err = config.etcd.KV.Put(
		context.TODO(),
		path.Join(
			PostalEtcdKeyPrefix,
			"networks", pool.ID.NetworkID,
			"pools", pool.ID.ID, "meta"),
		string(poolBytes),
	)

	if err != nil {
		return nil, err
	}

	return &etcdPoolManager{
		etcd: config.etcd,
		pool: pool,
		IPAM: IPAM,
	}, nil
}
