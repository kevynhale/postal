package ipam

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPv4ToUint(t *testing.T) {
	assert := assert.New(t)
	cases := []struct {
		addr   net.IP
		result uint32
	}{
		{
			addr:   net.ParseIP("10.10.10.10").To4(),
			result: 168430090,
		},
		{
			addr:   net.ParseIP("10.10.10.11").To4(),
			result: 168430091,
		},
		{
			addr:   net.ParseIP("255.255.255.255").To4(),
			result: 4294967295,
		},
	}

	for idx := range cases {
		assert.Equal(cases[idx].result, ipv4ToUint(cases[idx].addr))
		assert.Equal(cases[idx].addr, uintToIPv4(cases[idx].result))
	}
}

func TestIPv6ToUint(t *testing.T) {
	assert := assert.New(t)
	cases := []struct {
		addr   net.IP
		result [2]uint64
	}{
		{
			addr:   net.ParseIP("2620:0:2d0:200::10"),
			result: [2]uint64{0x2620000002d00200, 0x10},
		},
	}

	for idx := range cases {
		a, b := ipv6ToUint(cases[idx].addr)
		assert.Equal(cases[idx].result[0], a)
		assert.Equal(cases[idx].result[1], b)
		assert.Equal(cases[idx].addr, uintToIPv6(a, b))
	}
}
