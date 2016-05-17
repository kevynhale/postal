package postal

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestPoolsFilter(t *testing.T) {
	assert := assert.New(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	config := (&Config{}).WithEtcdClient(cli)
	network, err := config.NewNetwork(map[string]string{
		"example.com/networkName": "net1",
		"example.com/cluster":     "us-east-1",
	}, "172.16.0.0/16")
	assert.NoError(err)

	pool1, err := network.NewPool(map[string]string{
		"example.com/poolName": "default",
	}, 0, 1000, api.Pool_DYNAMIC)
	assert.NoError(err)

	_, err = network.NewPool(map[string]string{
		"example.com/poolName": "pool1",
	}, 0, 5, api.Pool_FIXED)
	assert.NoError(err)

	_, err = network.NewPool(map[string]string{
		"example.com/poolName": "pool2",
	}, 0, 5, api.Pool_FIXED)
	assert.NoError(err)

	pools, err := network.Pools(nil)
	assert.NoError(err)
	assert.Equal(3, len(pools))

	pools, err = network.Pools(map[string]string{"_id": pool1.ID()})
	assert.NoError(err)
	assert.Equal(1, len(pools))

	pools, err = network.Pools(map[string]string{"_network": network.APINetwork().ID})
	assert.NoError(err)
	assert.Equal(3, len(pools))

	pools, err = network.Pools(map[string]string{"_type": "fixed"})
	assert.NoError(err)
	assert.Equal(2, len(pools))

	pools, err = network.Pools(map[string]string{"example.com/poolName": "pool*"})
	assert.NoError(err)
	assert.Equal(2, len(pools))

	pools, err = network.Pools(map[string]string{"example.com/foo": ".*"})
	assert.NoError(err)
	assert.Equal(0, len(pools))

	pools, err = network.Pools(map[string]string{"example.com/poolName": "a(b"})
	assert.Error(err)
	assert.Equal(0, len(pools))

}
