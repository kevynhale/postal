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
	"strings"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/ipam"
	"github.com/twinj/uuid"
	"golang.org/x/net/context"
)

// PostalEtcdKeyPrefix defines the prefix used for all postal registry keys stored in etcd
const PostalEtcdKeyPrefix = "/postal/registry/v1/"

// Config is the base object which configures settings for postal internals
type Config struct {
	etcd *clientv3.Client
}

// WithEtcdClient is chaining method to set the etcdClient
func (config *Config) WithEtcdClient(etcd *clientv3.Client) *Config {
	config.etcd = etcd
	return config
}

type etcdNetworkMeta struct {
	ID          string            `json:"id"`
	IpamID      string            `json:"ipam"`
	Cidr        string            `json:"cidr"`
	Annotations map[string]string `json:"annotations"`
}

// Networks returns a list of network IDs the postal knows about
func (config *Config) Networks() ([]string, error) {
	resp, err := config.etcd.Get(context.TODO(), networksKey(), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	networks := []string{}
	for idx := range resp.Kvs {
		networks = append(networks, strings.Split(string(resp.Kvs[idx].Key), "/")[5])
	}

	return networks, nil
}

// Network returns a specific NetworkManager for a given ID
func (config *Config) Network(ID string) (NetworkManager, error) {
	resp, err := config.etcd.Get(context.TODO(), networkMetaKey(ID))
	if err != nil {
		return nil, err
	}

	network := &etcdNetworkMeta{}
	err = json.Unmarshal(resp.Kvs[0].Value, network)
	if err != nil {
		return nil, err
	}

	IPAM, err := ipam.FetchIPAM(network.IpamID, config.etcd)
	if err != nil {
		return nil, err
	}

	return &etcdNetworkManager{
		ID:          network.ID,
		cidr:        network.Cidr,
		annotations: network.Annotations,
		IPAM:        IPAM,
		etcd:        config.etcd,
	}, nil
}

// NewNetwork creates a new NetworkManager for the given block of addresses.
func (config *Config) NewNetwork(annotations map[string]string, cidr string) (NetworkManager, error) {
	IPAM, err := ipam.NewIPAM(cidr, config.etcd)
	if err != nil {
		return nil, err
	}

	network := &etcdNetworkMeta{
		ID:          uuid.NewV4().String(),
		IpamID:      IPAM.GetID(),
		Cidr:        cidr,
		Annotations: annotations,
	}

	networkBytes, err := json.Marshal(network)
	if err != nil {
		return nil, err
	}

	_, err = config.etcd.KV.Put(
		context.TODO(),
		networkMetaKey(network.ID),
		string(networkBytes),
	)

	if err != nil {
		//TODO: cleanup IPAM
		return nil, err
	}

	return &etcdNetworkManager{
		ID:          network.ID,
		cidr:        cidr,
		annotations: annotations,
		IPAM:        IPAM,
		etcd:        config.etcd,
	}, nil
}
