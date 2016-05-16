package postal

import (
	"fmt"
	"net"
	"path"
)

func networksKey() string {
	return path.Join(PostalEtcdKeyPrefix, "networks")
}

func networkMetaKey(ID string) string {
	return path.Join(networksKey(), ID)
}

func networkPoolsKey(ID string) string {
	return path.Join(PostalEtcdKeyPrefix, "network", ID, "pools")
}

func poolMetaKey(networkID, poolID string) string {
	return path.Join(networkPoolsKey(networkID), poolID)
}

func bindingListAddrKey(networkID string, addr net.IP) string {
	return path.Join(PostalEtcdKeyPrefix,
		"network", networkID,
		"bindings", canonicalIPString(addr),
	)
}

func bindingAddrKey(networkID string, addr net.IP) string {
	return path.Join(bindingListAddrKey(networkID, addr))
}

func bindingListKey(networkID, poolID string) string {
	return path.Join(
		PostalEtcdKeyPrefix,
		"network", networkID,
		"pool", poolID,
		"bindings",
	)
}

func bindingIDKey(networkID, poolID, bindingID string) string {
	return path.Join(
		bindingListKey(networkID, poolID), bindingID,
	)
}

func canonicalIPString(addr net.IP) string {
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

	if len(ret) == 0 {
		return ""
	}

	return ret[:len(ret)-1]
}
