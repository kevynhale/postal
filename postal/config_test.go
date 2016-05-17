package postal

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestNetworksFilter(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	config := (&Config{}).WithEtcdClient(cli)
	net1, err := config.NewNetwork(map[string]string{
		"example.com/networkName": "net1",
		"example.com/cluster":     "us-east-1",
	}, "172.16.0.0/16")
	assert.NoError(err)

	_, err = config.NewNetwork(map[string]string{
		"example.com/networkName": "net2",
		"example.com/cluster":     "us-east-1",
	}, "172.17.0.0/16")
	assert.NoError(err)

	_, err = config.NewNetwork(map[string]string{
		"example.com/networkName": "net3",
		"example.com/cluster":     "us-west-1",
	}, "172.20.0.0/16")
	assert.NoError(err)

	networks, err := config.Networks(nil)
	assert.NoError(err)
	assert.Equal(3, len(networks))

	networks, err = config.Networks(map[string]string{"example.com/networkName": "net5"})
	assert.NoError(err)
	assert.Equal(0, len(networks))

	networks, err = config.Networks(map[string]string{"example.com/networkName": "net1"})
	assert.NoError(err)
	assert.Equal(1, len(networks))

	networks, err = config.Networks(map[string]string{"example.com/cluster": "us-east-1"})
	assert.NoError(err)
	assert.Equal(2, len(networks))

	networks, err = config.Networks(map[string]string{"example.com/cluster": "us*"})
	assert.NoError(err)
	assert.Equal(3, len(networks))

	networks, err = config.Networks(map[string]string{"_id": net1.APINetwork().ID})
	assert.NoError(err)
	assert.Equal(1, len(networks))

	networks, err = config.Networks(map[string]string{"_cidr": "172*"})
	assert.NoError(err)
	assert.Equal(3, len(networks))

	networks, err = config.Networks(map[string]string{"foo": ".*"})
	assert.NoError(err)
	assert.Equal(0, len(networks))

	networks, err = config.Networks(map[string]string{"example.com/cluster": ".(*"})
	assert.Error(err)
	assert.Equal(0, len(networks))
}
