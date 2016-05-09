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
	"encoding/json"
	"fmt"
	"math"
	"net"
	"path"
	"sync"

	"golang.org/x/net/context"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/twinj/uuid"
)

// IpamEtcdKeyPrefix defines the prefix used for all ipam keys stored in etcd
const IpamEtcdKeyPrefix = "/postal/ipam/v1/"

const (
	// MinIPv4SubnetSize is the smallest block that we will allocate and track for ipv4 addresses.
	MinIPv4SubnetSize = 24
	// MinIPv6SubnetSize is the smallest block that we will allocate and track for ipv6 addresses.
	MinIPv6SubnetSize = 112
	// PostalIPAMRetryMax is the max number of times a retry for an allocation should be attepted.
	PostalIPAMRetryMax = 10
)

var MinIPv4SubnetMask = net.IPv4Mask(255, 255, 255, 0)
var IPv4_32SubnetMask = net.IPv4Mask(255, 255, 255, 255)
var MinIPv6SubnetMask = net.CIDRMask(112, 128)
var IPv6_128SubnetMask = net.CIDRMask(128, 128)

// IPAM defines the interface for allocating blocks of addresses
type IPAM interface {
	// Allocate a number of addresses. These are returned as one or more CIDR blocks.
	Allocate(addresses uint) ([]net.IPNet, error)
	// Release a specific address back.
	Release(net.IP) error
	// Claim forces a claim on a specific address.
	// If the requested address has already been allocated, this will return an error
	Claim(net.IP) error
	// IsAvailable checks to see if a specifc IP as been allocated.
	IsAvailable(net.IP) bool
	// Size returns the cardinality of the set of addresses the IPAM object tracks.
	Size() uint64
	// Available returns the cardinality of the non-allocated set of addresses.
	Available() uint64
	// GetID is the unique identifier for the ipam module
	GetID() string
}

// ipamEtcdBlock wraps the individual ipam block with etcd specific attributes
type ipamEtcdBlock struct {
	block   *ipamBlock
	key     string
	version int64
}

// Cmp returns a slice or etcd comparison operations for use in key transactions.
func (block *ipamEtcdBlock) Cmp() []etcd.Cmp {
	return []etcd.Cmp{
		etcd.Compare(etcd.Version(block.key), "=", block.version),
	}
}

// PutOp returns a slice or etcd put operations for use in key transactions.
func (block *ipamEtcdBlock) PutOp() []etcd.Op {
	blockJSON, _ := block.block.MarshalJSON()
	return []etcd.Op{
		etcd.OpPut(block.key, string(blockJSON)),
	}
}

type etcdIPAM struct {
	ID          string
	net         *net.IPNet
	etcd        *etcd.Client
	nextKey     string
	nextKeyLock sync.Locker
}

// FetchIPAM fetches the IPAM object for the given ID.
func FetchIPAM(ID string, client *etcd.Client) (IPAM, error) {
	resp, err := client.KV.Get(context.TODO(), path.Join(IpamEtcdKeyPrefix, ID, "cidr"))
	if err != nil {
		return nil, err
	}

	_, ipnet, _ := net.ParseCIDR(string(resp.Kvs[0].Value))
	resp, err = client.KV.Get(context.TODO(), path.Join(IpamEtcdKeyPrefix, ID, "nextKey"))
	if err != nil {
		return nil, err
	}

	i := &etcdIPAM{
		ID:          ID,
		net:         ipnet,
		etcd:        client,
		nextKey:     string(resp.Kvs[0].Value),
		nextKeyLock: &sync.Mutex{},
	}

	return i, nil
}

// NewIPAM takes a cidr block and etcd client and returns an implementaton of the IPAM interface.
// If the cidr block is larger than the MinBlockSize for the givcen address family,
// the IPAM module will divide the block into multiple sublocks the size of MinBlockSize.
func NewIPAM(cidr string, client *etcd.Client) (IPAM, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	i := &etcdIPAM{
		ID:          uuid.NewV4().String(),
		net:         ipnet,
		etcd:        client,
		nextKey:     ipnet.IP.String(),
		nextKeyLock: &sync.Mutex{},
	}

	resp, err := client.KV.Txn(context.TODO()).If(
		etcd.Compare(etcd.Version(path.Join(IpamEtcdKeyPrefix, i.ID, "nextKey")), "=", 0),
	).Then(
		etcd.OpPut(
			path.Join(IpamEtcdKeyPrefix, i.ID, "nextKey"),
			i.nextKey,
		),
		etcd.OpPut(path.Join(IpamEtcdKeyPrefix, i.ID, "cidr"), cidr),
	).Commit()

	if err != nil {
		return nil, err
	}

	if resp.Succeeded {
		return i, nil
	}

	return nil, fmt.Errorf("ipam: failed to persist IPAM to datastore")
}

func (ipam *etcdIPAM) String() string {
	return fmt.Sprintf("ID: %s, net: %v, nextKey: %s", ipam.ID, ipam.net, ipam.nextKey)
}

func (ipam *etcdIPAM) Allocate(addresses uint) ([]net.IPNet, error) {
	retryCount := 0
ALLOCATE:
	// fetch list of provisioned blocks
	blocks, _ := ipam.fetchIpamBlocks()

	// allocatedBlocks holds the set of addresses to be returned to the caller.
	allocatedBlocks := []net.IPNet{}

	// toCommit holds the set of ipamBlocks that need to be commited to the etcd.
	toCommit := []*ipamEtcdBlock{}

	for _, block := range blocks {
		// check to see if we've allocated all the addresses we need
		if uint(len(allocatedBlocks)) == addresses {
			break
		}
		// check if there are any addresses availble in the ipamBlock
		if block.block.Available() == 0 {
			continue
		}

		// if the number of available addresses is smaller than what is required we'll claim whats left of the it
		// otherwise only allocate the number of required addresses.
		if block.block.Available() < (addresses - uint(len(allocatedBlocks))) {
			allocatedBlocks = append(allocatedBlocks, ipam.allocateSubBlock(block.block.Available(), block.block)...)
		} else {
			allocatedBlocks = append(allocatedBlocks, ipam.allocateSubBlock((addresses-uint(len(allocatedBlocks))), block.block)...)
		}

		// we've touched this ipamBlock, so push it onto the list to be commited.
		toCommit = append(toCommit, block)
	}

	// if after iterating through the provisioned ipamBlock doesn't yield enough addresses
	// a new ipamBlock must be provisoned.
	for uint(len(allocatedBlocks)) < addresses {
		block, err := ipam.nextBlock()
		if err != nil {
			if retryCount < PostalIPAMRetryMax {
				retryCount++
				goto ALLOCATE
			} else {
				return nil, err
			}
		}

		if block.block.Available() < (addresses - uint(len(allocatedBlocks))) {
			allocatedBlocks = append(allocatedBlocks, ipam.allocateSubBlock(block.block.Available(), block.block)...)
		} else {
			allocatedBlocks = append(allocatedBlocks, ipam.allocateSubBlock((addresses-uint(len(allocatedBlocks))), block.block)...)
		}
		toCommit = append(toCommit, block)
	}

	cmps := []etcd.Cmp{}
	ops := []etcd.Op{}
	for _, block := range toCommit {
		cmps = append(cmps, block.Cmp()...)
		ops = append(ops, block.PutOp()...)
	}

	resp, err := ipam.etcd.KV.Txn(context.Background()).If(cmps...).Then(ops...).Commit()
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		//TODO:backoff/rety count
		goto ALLOCATE
	}

	return allocatedBlocks, nil
}

// nextBlock provisions the next block of addresses from the IPAM module.
func (ipam *etcdIPAM) nextBlock() (*ipamEtcdBlock, error) {
	ipam.nextKeyLock.Lock()
	ip := net.ParseIP(ipam.nextKey)
	ipam.nextKeyLock.Unlock()
	var newNextIP net.IP
	var block *ipamEtcdBlock
	if len(ipam.net.IP) == net.IPv4len {
		dec := ipv4ToUint(ip.To4())
		newNextIP = uintToIPv4(dec + uint32(math.Pow(2, 8)))
		block = &ipamEtcdBlock{
			block: ipamBlockInit(
				&net.IPNet{
					IP:   ip.To4(),
					Mask: MinIPv4SubnetMask,
				},
			),
			key:     path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations", ip.String()),
			version: int64(0),
		}
	} else {
		pre, sub := ipv6ToUint(ip)
		newNextIP = uintToIPv6(pre, uint64(math.Pow(float64(sub)+2, 16)))
		block = &ipamEtcdBlock{
			block: ipamBlockInit(
				&net.IPNet{
					IP:   ip,
					Mask: MinIPv6SubnetMask,
				},
			),
			key:     path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations", ip.String()),
			version: int64(0),
		}
	}

	blockBytes, err := json.Marshal(block.block)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("TXN: if version(%s) = 0 and nextKey = %s then create allocation(%s) and update nextKey = %s\n", ip.String(), ip.String(), ip.String(), newNextIP.String())
	resp, err := ipam.etcd.KV.Txn(context.Background()).If(
		etcd.Compare(etcd.Version(path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations", ip.String())), "=", 0),
		etcd.Compare(etcd.Value(path.Join(IpamEtcdKeyPrefix, ipam.ID, "nextKey")), "=", ip.String()),
	).Then(
		etcd.OpPut(
			path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations", ip.String()),
			string(blockBytes),
		),
		etcd.OpPut(
			path.Join(IpamEtcdKeyPrefix, ipam.ID, "nextKey"),
			newNextIP.String(),
		),
	).Commit()
	//fmt.Println("Setting next block to: ", nextIP.String())

	if err != nil {
		return nil, err
	}

	if resp.Succeeded == false {
		return nil, fmt.Errorf("ipam: failed to allocate new block=%s", newNextIP.String())
	}

	ipam.nextKeyLock.Lock()
	ipam.nextKey = newNextIP.String()
	ipam.nextKeyLock.Unlock()

	return block, nil
}

func (ipam *etcdIPAM) maxAllocations() float64 {
	ones, _ := ipam.net.Mask.Size()
	if len(ipam.net.IP) == net.IPv4len {
		return math.Pow(float64(2), float64(MinIPv4SubnetSize-ones))
	}
	return math.Pow(float64(2), float64(MinIPv6SubnetSize-ones))
}

func (ipam *etcdIPAM) allocateSubBlock(addresses uint, block *ipamBlock) []net.IPNet {
	allocatedBlocks := []net.IPNet{}
	addrs := block.BulkRequest(addresses)
	for _, addr := range addrs {
		if addr4 := addr.To4(); len(addr4) == net.IPv4len {
			allocatedBlocks = append(allocatedBlocks, net.IPNet{
				IP:   addr4,
				Mask: IPv4_32SubnetMask,
			})
		} else {
			allocatedBlocks = append(allocatedBlocks, net.IPNet{
				IP:   addr,
				Mask: IPv6_128SubnetMask,
			})
		}
	}
	return allocatedBlocks
}

func (ipam *etcdIPAM) fetchIpamBlocks() (map[string]*ipamEtcdBlock, error) {
	resp, err := ipam.etcd.KV.Get(context.Background(), path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations"), etcd.WithPrefix())
	if err != nil {
		return nil, err
	}
	blocks := map[string]*ipamEtcdBlock{}
	for idx := range resp.Kvs {
		block := &ipamBlock{}
		json.Unmarshal(resp.Kvs[idx].Value, block)
		etcdBlock := &ipamEtcdBlock{
			block:   block,
			key:     string(resp.Kvs[idx].Key),
			version: resp.Kvs[idx].Version,
		}
		blocks[string(resp.Kvs[idx].Key)] = etcdBlock
	}
	return blocks, nil
}

func (ipam *etcdIPAM) fetchIpamBlock(addr string) (*ipamEtcdBlock, error) {
	resp, err := ipam.etcd.KV.Get(context.Background(), path.Join(IpamEtcdKeyPrefix, ipam.ID, "allocations", addr))
	if err != nil {
		return nil, err
	}
	block := &ipamBlock{}
	json.Unmarshal(resp.Kvs[0].Value, block)
	etcdBlock := &ipamEtcdBlock{
		block:   block,
		key:     string(resp.Kvs[0].Key),
		version: resp.Kvs[0].Version,
	}
	return etcdBlock, nil
}

func (ipam *etcdIPAM) Release(ip net.IP) error {
	var block *ipamEtcdBlock
	var err error
RELEASE:
	if len(ipam.net.IP) == net.IPv4len {
		block, err = ipam.fetchIpamBlock(ip.Mask(MinIPv4SubnetMask).String())
	} else {
		block, err = ipam.fetchIpamBlock(ip.Mask(MinIPv6SubnetMask).String())
	}
	if err != nil {
		return err
	}

	block.block.Release(ip)

	resp, err := ipam.etcd.KV.Txn(context.TODO()).If(block.Cmp()...).Then(block.PutOp()...).Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		//TODO:backoff/rety count
		goto RELEASE
	}

	return nil
}

func (ipam *etcdIPAM) Claim(ip net.IP) error {
	var block *ipamEtcdBlock
	var err error
CLAIM:
	if len(ipam.net.IP) == net.IPv4len {
		block, err = ipam.fetchIpamBlock(ip.Mask(MinIPv4SubnetMask).String())
	} else {
		block, err = ipam.fetchIpamBlock(ip.Mask(MinIPv6SubnetMask).String())
	}
	if err != nil {
		return err
	}

	claimed := block.block.Claim(ip)
	if !claimed {
		return fmt.Errorf("ipam/claim: addr already claimed: %s", ip.String())
	}

	resp, err := ipam.etcd.KV.Txn(context.TODO()).If(block.Cmp()...).Then(block.PutOp()...).Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		//TODO:backoff/rety count
		goto CLAIM
	}

	return nil
}

func (ipam *etcdIPAM) IsAvailable(ip net.IP) bool {
	return true
}

func (ipam *etcdIPAM) Size() uint64 {
	return uint64(ipam.maxAllocations())
}

func (ipam *etcdIPAM) Available() uint64 {
	return uint64(0)
}

func (ipam *etcdIPAM) GetID() string {
	return ipam.ID
}
