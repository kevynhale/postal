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

import "net"

func ipv4ToUint(addr net.IP) uint32 {
	var sum uint32
	for i := uint32(0); i < net.IPv4len; i++ {
		sum += uint32(addr[i]) << (24 - i*8)
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
