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

	"regexp"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/ipam"
	"github.com/pkg/errors"
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

// Networks returns a list of filtered networks
func (config *Config) Networks(filters map[string]string) ([]*api.Network, error) {
	resp, err := config.etcd.Get(context.TODO(), networksKey(), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	noFilter := filters == nil || len(filters) == 0

	networks := []*api.Network{}
	for idx := range resp.Kvs {
		network := &api.Network{}
		err = json.Unmarshal(resp.Kvs[idx].Value, network)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal network")
		}

		if noFilter {
			networks = append(networks, network)
		} else {
			var matched bool
			for field, filter := range filters {
				switch field {
				case "_id":
					matched, err = regexp.MatchString(filter, network.ID)
				case "_cidr":
					matched, err = regexp.MatchString(filter, network.Cidr)
				default:
					if val, ok := network.Annotations[field]; ok {
						matched, err = regexp.MatchString(filter, val)
					} else {
						break
					}
				}
				if err != nil {
					return nil, errors.Wrapf(err, "failed to compile filter '%s'", filter)
				}

				if !matched {
					break
				}
			}

			if matched {
				networks = append(networks, network)
			}
		}
	}

	return networks, nil
}

// Pools returns a list of filtered pools
func (config *Config) Pools(filters map[string]string) ([]*api.Pool, error) {
	networks, err := config.Networks(nil)
	if err != nil {
		return nil, err
	}

	pools := []*api.Pool{}

	for idx := range networks {
		nm, err := config.Network(networks[idx].ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network")
		}

		p, err := nm.Pools(filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network")
		}

		pools = append(pools, p...)
	}

	return pools, nil
}

// Network returns a specific NetworkManager for a given ID
func (config *Config) Network(ID string) (NetworkManager, error) {
	resp, err := config.etcd.Get(context.TODO(), networkMetaKey(ID))
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) != 1 {
		return nil, errors.New("postal: network could not be found")
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
		ID:          newNetworkID(),
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
