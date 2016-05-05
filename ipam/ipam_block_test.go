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
	"math"
	"net"
	"testing"

	tassert "github.com/stretchr/testify/assert"
)

func TestIpRelease(t *testing.T) {
	assert := tassert.New(t)
	count := 25
	testAddress, ipNet, _ := net.ParseCIDR("192.168.0.0/24")
	testAddress = testAddress.To4()
	manager := ipamBlockInit(ipNet)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		testAddress[3] = byte(i)
		assert.Equal(testAddress, address)
	}

	manager.Release(net.ParseIP("192.168.0.1"))
	manager.Release(net.ParseIP("192.168.0.11"))

	address := manager.Request()
	testAddress[3] = 25
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 26
	assert.Equal(testAddress, address)
}

func TestIpv6Release(t *testing.T) {
	assert := tassert.New(t)
	count := 25
	testAddress, ipNet, _ := net.ParseCIDR("2001:db8::/112")
	manager := ipamBlockInit(ipNet)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		testAddress[15] = byte(i)
		assert.Equal(testAddress, address)
	}

	manager.Release(net.ParseIP("2001:db8::1"))
	manager.Release(net.ParseIP("2001:db8::b"))

	address := manager.Request()
	assert.Equal("2001:db8::19", address.String())
	address = manager.Request()
	assert.Equal("2001:db8::1a", address.String())
}

func TestIpTicking(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("10.0.0.0/29")
	testAddress = testAddress.To4()
	manager := ipamBlockInit(ipNet)
	var address net.IP
	address = manager.Request()

	testAddress[3] = 1
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 2
	assert.Equal(testAddress, address)

	manager.Release(net.ParseIP("10.0.0.1"))
	address = manager.Request()
	testAddress[3] = 3
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 4
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 5
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 6
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 1
	assert.Equal(testAddress, address)

	manager.Release(net.ParseIP("10.0.0.1"))
	manager.Release(net.ParseIP("10.0.0.3"))
	address = manager.Request()
	testAddress[3] = 3
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[3] = 1
	assert.Equal(testAddress, address)

}

func TestIpv6Ticking(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("2001:db8::/125")
	manager := ipamBlockInit(ipNet)
	var address net.IP
	address = manager.Request()
	testAddress[15] = 1
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 2
	assert.Equal(testAddress, address)

	manager.Release(net.ParseIP("2001:db8::1"))
	address = manager.Request()
	testAddress[15] = 3
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 4
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 5
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 6
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 1
	assert.Equal(testAddress, address)

	manager.Release(net.ParseIP("2001:db8::1"))
	manager.Release(net.ParseIP("2001:db8::3"))
	address = manager.Request()
	testAddress[15] = 3
	assert.Equal(testAddress, address)

	address = manager.Request()
	testAddress[15] = 1
	assert.Equal(testAddress, address)

}

func TestIpClaim(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("10.10.2.0/24")
	testAddress = testAddress.To4()
	testAddress[3] = 1
	manager := ipamBlockInit(ipNet)
	manager.Claim(testAddress)
	var address net.IP
	address = manager.Request()
	assert.NotEqual(testAddress, address)
}

func TestIpv6Claim(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("2001:db8::/112")
	testAddress[15] = 1
	manager := ipamBlockInit(ipNet)
	manager.Claim(testAddress)
	var address net.IP
	address = manager.Request()
	assert.NotEqual(testAddress, address)
}

func TestBadIpClaim(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("10.10.2.0/24")
	testAddress = testAddress.To4()
	testAddress[3] = 1
	manager := ipamBlockInit(ipNet)
	ip := net.ParseIP("10.220.2.1")
	assert.False(manager.Claim(ip))
	var address net.IP
	address = manager.Request()
	assert.Equal(testAddress, address)
}

func TestBadIpv6Claim(t *testing.T) {
	assert := tassert.New(t)
	testAddress, ipNet, _ := net.ParseCIDR("2001:db8::/112")
	testAddress[15] = 1
	manager := ipamBlockInit(ipNet)
	ip := net.ParseIP("2001:db8::")
	assert.False(manager.Claim(ip))
	var address net.IP
	address = manager.Request()
	assert.Equal(testAddress, address)
}

func TestGetIpFullMask(t *testing.T) {
	count := 300
	_, ipNet, _ := net.ParseCIDR("192.168.0.0/16")
	manager := ipamBlockInit(ipNet)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		address = address.To4()
		if i%256 != int(address[3]) || i/256 != int(address[2]) {
			t.Error(address.String())
		}
	}
}

func TestGetIpFullNetwork(t *testing.T) {
	count := 255
	_, ipNet, _ := net.ParseCIDR("192.168.0.0/24")
	manager := ipamBlockInit(ipNet)
	//IPAMPrint(*ipNet, mask)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		address = address.To4()
		if i%256 != int(address[3]) || i/256 != int(address[2]) {
			t.Error(address.String())
		}
	}
	address := manager.Request()
	if address != nil {
		t.Error(address)
	}
}

func TestGetIpv6FullNetwork(t *testing.T) {
	assert := tassert.New(t)
	count := 255
	testAddress, ipNet, _ := net.ParseCIDR("2001:db8::/120")
	manager := ipamBlockInit(ipNet)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		testAddress[15] = byte(i)
		assert.Equal(testAddress, address)
	}
	address := manager.Request()
	if address != nil {
		t.Error(address)
	}
}

func TestGetIpPartialMask(t *testing.T) {
	count := 300
	_, ipNet, _ := net.ParseCIDR("192.169.32.0/20")
	manager := ipamBlockInit(ipNet)
	for i := 1; i < count; i++ {
		var address net.IP
		address = manager.Request()
		address = address.To4()
		if i%256 != int(address[3]) || 32+i/256 != int(address[2]) {
			t.Error(address.String())
		}
	}
}

func TestSize(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("192.168.0.0/16")
	manager := ipamBlockInit(ipNet)
	if manager.Size() != uint(math.Exp2(16)) {
		t.Error(manager.Size())
	}
}

func TestV6Size(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("2001:db8::/112")
	manager := ipamBlockInit(ipNet)
	tassert.Equal(t, uint(math.Exp2(16)), manager.Size())
}

func TestAvailable(t *testing.T) {
	count := uint(253)
	_, ipNet, _ := net.ParseCIDR("172.30.32.0/24")
	manager := ipamBlockInit(ipNet)
	for i := uint(0); i < count; i++ {
		manager.Request()
	}
	if manager.Available() != 1 {
		t.Error(manager.Available())
	}
}

func TestBulkRequest(t *testing.T) {
	assert := tassert.New(t)
	_, ipNet, _ := net.ParseCIDR("172.30.32.0/24")
	manager := ipamBlockInit(ipNet)
	addrs := manager.BulkRequest(uint(100))
	assert.Equal(100, len(addrs))

}

func TestV6Available(t *testing.T) {
	count := uint(253)
	_, ipNet, _ := net.ParseCIDR("2001:db8::/120")
	manager := ipamBlockInit(ipNet)
	for i := uint(0); i < count; i++ {
		manager.Request()
	}
	tassert.Equal(t, uint(1), manager.Available())
}

func benchmarkIPRequest(ipNet *net.IPNet, b *testing.B) {
	manager := ipamBlockInit(ipNet)
	for n := 0; n < b.N; n++ {
		manager.Request()
	}
}

func BenchmarkIPRequest28(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/28")
	benchmarkIPRequest(ipNet, b)
}

func BenchmarkIPRequest24(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/24")
	benchmarkIPRequest(ipNet, b)
}

func BenchmarkIPRequest20(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/20")
	benchmarkIPRequest(ipNet, b)
}

func BenchmarkIPRequest16(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/16")
	benchmarkIPRequest(ipNet, b)
}

func BenchmarkIPV6Request112(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("2001:db8::/112")
	benchmarkIPRequest(ipNet, b)
}
