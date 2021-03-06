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
	"fmt"
	"math"
	"net"
)

func ipv4ToUint(addr net.IP) uint32 {
	if addr.To4() == nil {
		return 0
	}

	var sum uint32
	for i := uint32(0); i < net.IPv4len; i++ {
		sum += uint32(addr.To4()[i]) << (24 - i*8)
	}

	return sum
}

func uintToIPv4(num uint32) net.IP {
	return net.IP{
		byte((num >> 24) & 0xff),
		byte((num >> 16) & 0xff),
		byte((num >> 8) & 0xff),
		byte(num & 0xff)}
}

func ipv6ToUint(addr net.IP) (uint64, uint64) {
	var prefix, subnet uint64
	for i := uint64(0); i < net.IPv6len/2; i++ {
		prefix += uint64(addr[i]) << (56 - i*8)
		subnet += uint64(addr[i+8]) << (56 - i*8)
	}

	return prefix, subnet
}

func uintToIPv6(prefix, subnet uint64) net.IP {
	return net.IP{
		byte((prefix >> 56) & 0xff),
		byte((prefix >> 48) & 0xff),
		byte((prefix >> 40) & 0xff),
		byte((prefix >> 32) & 0xff),
		byte((prefix >> 24) & 0xff),
		byte((prefix >> 16) & 0xff),
		byte((prefix >> 8) & 0xff),
		byte(prefix & 0xff),
		byte((subnet >> 56) & 0xff),
		byte((subnet >> 48) & 0xff),
		byte((subnet >> 40) & 0xff),
		byte((subnet >> 32) & 0xff),
		byte((subnet >> 24) & 0xff),
		byte((subnet >> 16) & 0xff),
		byte((subnet >> 8) & 0xff),
		byte(subnet & 0xff),
	}
}

// CanonicalIPString takes an ip and returns a string used in the etcd key
func CanonicalIPString(addr net.IP) string {
	ret := ""
	if addr.To4() != nil {
		for _, b := range addr.To4() {
			ret += fmt.Sprintf("%03d/", b)
		}
	} else {
		for i, b := range addr {
			ret += fmt.Sprintf("%02x", b)
			if i%2 == 1 {
				ret += "/"
			}
		}
	}
	return ret[:len(ret)-1]
}

func lastCIDRAddr(ipnet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)
	ones, bits := ipnet.Mask.Size()
	size := math.Pow(float64(2), float64(bits-ones))
	for i := float64(0); i < size-1; i++ {
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] > 0 {
				break
			}
		}
	}
	return ip
}
