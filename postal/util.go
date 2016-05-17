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

func mergeMap(base, merge map[string]string) map[string]string {
	for k, v := range merge {
		base[k] = v
	}
	return base
}
