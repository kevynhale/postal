package postal

import (
	"net"
	"path"
	"regexp"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

var (
	bindingKeyRegex, _ = regexp.Compile(`/postal/registry/v1/network/([0-9a-z-]+)/bindings/(.*)`)
)

type Janitor struct {
	etcd *clientv3.Client
}

func NewJanitor(cli *clientv3.Client) *Janitor {
	return &Janitor{etcd: cli}
}

func (j *Janitor) Run() {
	config := (&Config{}).WithEtcdClient(j.etcd)
	watcher := j.etcd.Watch(context.TODO(), path.Join(PostalEtcdKeyPrefix, "network"), clientv3.WithPrefix())
	for wresp := range watcher {
		for _, ev := range wresp.Events {
			if ev.Type.String() == "DELETE" && bindingKeyRegex.MatchString(string(ev.Kv.Key)) {
				match := bindingKeyRegex.FindStringSubmatch(string(ev.Kv.Key))
				if len(match) != 3 {
					continue
				}
				nm, err := config.Network(match[1])
				if err != nil {
					continue
				}

				var ip net.IP

				cIP := strings.Split(match[2], "/")
				if len(cIP) == 4 {
					ip = net.ParseIP(strings.Join(cIP, "."))

				} else {
					ip = net.ParseIP(strings.Join(cIP, ":"))
				}

				plog.Debugf("janitor: cleaning up %s for network %s: %v", ip.String(), nm.APINetwork().ID,
					nm.(*etcdNetworkManager).IPAM.Release(ip))
			}
		}
	}
}
