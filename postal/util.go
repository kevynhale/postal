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

func bindingAddrKey(networkID, bindingID string, addr net.IP) string {
	return path.Join(bindingListAddrKey(networkID, addr), bindingID)
}

func bindingIDKey(networkID, poolID, bindingID string) string {
	return path.Join(
		PostalEtcdKeyPrefix,
		"network", networkID,
		"pool", poolID,
		"bindings", bindingID,
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
	return ret[:len(ret)-1]
}
