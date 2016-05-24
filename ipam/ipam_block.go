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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
)

type ipamBlock struct {
	Subnet    net.IPNet
	key       string
	bitset    []byte
	tick      uint
	allocated uint
}

func (ipam *ipamBlock) Request() net.IP {
	if ipam.Available() == 0 {
		return nil
	}

	for pos := ipam.tick + 1; pos < uint(len(ipam.bitset)*8); pos++ {
		if !testBit(ipam.bitset, pos) {
			setBit(ipam.bitset, pos)
			ipam.tick = pos
			ipam.allocated = ipam.allocated + 1
			return getIP(ipam.Subnet, pos)
		}
	}

	pos := testAndSetBit(ipam.bitset)
	if pos == uint(len(ipam.bitset)*8) {
		return nil
	}
	ipam.tick = pos
	ipam.allocated = ipam.allocated + 1
	return getIP(ipam.Subnet, pos)
}

func (ipam *ipamBlock) BulkRequest(addresses uint) []net.IP {
	if addresses > ipam.Available() {
		return nil
	}
	addrs := []net.IP{}
	for i := uint(0); i < addresses; i++ {
		addrs = append(addrs, ipam.Request())
	}
	return addrs
}

func (ipam *ipamBlock) Release(address net.IP) {
	pos := getBitPosition(address, ipam.Subnet)
	if testBit(ipam.bitset, pos) {
		clearBit(ipam.bitset, pos)
		ipam.allocated = ipam.allocated - 1
	}
}

func (ipam *ipamBlock) Claim(address net.IP) bool {
	if !ipam.Subnet.Contains(address) {
		return false
	}

	pos := getBitPosition(address, ipam.Subnet)
	if testBit(ipam.bitset, pos) {
		return false
	}

	setBit(ipam.bitset, pos)
	ipam.allocated = ipam.allocated + 1
	return true
}

func (ipam *ipamBlock) Size() uint {
	return uint(bitCount(ipam.Subnet))
}

func (ipam *ipamBlock) Available() uint {
	return ipam.Size() - ipam.allocated
}

func ipamBlockInit(ipnet *net.IPNet, setFirst, setLast bool) *ipamBlock {
	subnet := *ipnet
	mask := make([]byte, int(bitCount(subnet)/8))
	allocated := uint(0)
	if setFirst {
		setBit(mask, 0)
		allocated++
	}
	if setLast {
		setBit(mask, uint(len(mask)*8)-1)
		allocated++
	}
	return &ipamBlock{
		Subnet:    subnet,
		bitset:    mask,
		allocated: allocated,
	}
}

func (ipam *ipamBlock) Print(w io.Writer) {
	for i := uint(0); i < uint(len(ipam.bitset)*8); i++ {
		fmt.Fprintf(w, "%s-%t ", getIP(ipam.Subnet, i).String(), testBit(ipam.bitset, i))
	}
}

func getBitPosition(address net.IP, subnet net.IPNet) uint {
	mask, size := subnet.Mask.Size()
	if address.To4() != nil {
		address = address.To4()
	}
	tb := size / 8
	byteCount := (size - mask) / 8
	bitCount := (size - mask) % 8
	pos := uint(0)

	for i := 0; i <= byteCount; i++ {
		maskLen := 0xFF
		if i == byteCount {
			if bitCount != 0 {
				maskLen = int(math.Pow(2, float64(bitCount))) - 1
			} else {
				maskLen = 0
			}
		}
		pos += (uint(address[tb-i-1]) & uint(0xFF&maskLen)) << uint(8*i)
	}
	return pos
}

// Given Subnet of interest and free bit position, this method returns the corresponding ip address
// This method is functional and tested. Refer to ipam_test.go But can be improved

func getIP(subnet net.IPNet, pos uint) net.IP {
	retAddr := make([]byte, len(subnet.IP))
	copy(retAddr, subnet.IP)

	mask, _ := subnet.Mask.Size()
	var tb, byteCount, bitCount int
	if subnet.IP.To4() != nil {
		tb = 4
		byteCount = (32 - mask) / 8
		bitCount = (32 - mask) % 8
	} else {
		tb = 16
		byteCount = (128 - mask) / 8
		bitCount = (128 - mask) % 8
	}
	for i := 0; i <= byteCount; i++ {
		maskLen := 0xFF
		if i == byteCount {
			if bitCount != 0 {
				maskLen = int(math.Pow(2, float64(bitCount))) - 1
			} else {
				maskLen = 0
			}
		}
		masked := pos & uint((0xFF&maskLen)<<uint(8*i))
		retAddr[tb-i-1] |= byte(masked >> uint(8*i))
	}
	return net.IP(retAddr)
}

func bitCount(addr net.IPNet) float64 {
	mask, _ := addr.Mask.Size()
	if addr.IP.To4() != nil {
		return math.Pow(2, float64(32-mask))
	}
	return math.Pow(2, float64(128-mask))
}

func setBit(a []byte, k uint) {
	a[k/8] |= 1 << (k % 8)
}

func clearBit(a []byte, k uint) {
	a[k/8] &= ^(1 << (k % 8))
}

func testBit(a []byte, k uint) bool {
	return ((a[k/8] & (1 << (k % 8))) != 0)
}

func testAndSetBit(a []byte) uint {
	var i uint
	for i = uint(0); i < uint(len(a)*8); i++ {
		if !testBit(a, i) {
			setBit(a, i)
			return i
		}
	}
	return i
}

type ipamBlockJSON struct {
	Subnet    string `json:"subnet"`
	Bitset    string `json:"bitset"`
	Tick      uint   `json:"tick"`
	Allocated uint   `json:"allocated"`
}

// json.Marshaler impl
func (ipam ipamBlock) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(ipamBlockJSON{
		Subnet:    ipam.Subnet.String(),
		Bitset:    hex.EncodeToString(ipam.bitset),
		Tick:      ipam.tick,
		Allocated: ipam.allocated,
	})
	return b, err
}

// json.Unmarshaler impl
func (ipam *ipamBlock) UnmarshalJSON(j []byte) error {
	ipamjson := &ipamBlockJSON{}
	err := json.Unmarshal(j, ipamjson)
	if err != nil {
		return err
	}
	_, ipNet, _ := net.ParseCIDR(ipamjson.Subnet)
	ipam.Subnet = *ipNet

	bits, err := hex.DecodeString(ipamjson.Bitset)
	if err != nil {
		return err
	}
	ipam.bitset = bits
	ipam.tick = ipamjson.Tick
	ipam.allocated = ipamjson.Allocated

	return nil
}
