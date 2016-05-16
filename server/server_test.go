package server

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type sandboxedServerTest func(assert *assert.Assertions, client api.PostalClient)

func (srvTest sandboxedServerTest) execute(t *testing.T) {
	assert := assert.New(t)

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	assert.NoError(err)

	defer cli.Close()
	defer cli.KV.Delete(context.Background(), "/", clientv3.WithPrefix())

	serverAddr := "127.0.0.1:54321"

	lis, err := net.Listen("tcp", serverAddr)
	defer lis.Close()
	assert.NoError(err)

	grpcServer := grpc.NewServer()
	srv := PostalServer{etcd: cli}
	srv.Register(grpcServer)
	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	assert.NoError(err)
	defer conn.Close()
	client := api.NewPostalClient(conn)
	srvTest(assert, client)
}

func TestSrvNetwork(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {
		// Add sme networks to the server
		networkCount := 5
		networks := map[string]*api.Network{}
		for i := 0; i < networkCount; i++ {
			cidr := fmt.Sprintf("10.%d.0.0/16", i)
			resp, err := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
				Annotations: map[string]string{
					"foo":      "bar",
					"netCount": string(i),
				},
				Cidr: cidr,
			})
			assert.NoError(err)
			assert.Equal(cidr, resp.Network.Cidr)
			assert.Equal("bar", resp.Network.Annotations["foo"])
			assert.Equal(string(i), resp.Network.Annotations["netCount"])
			networks[resp.Network.ID] = resp.Network
		}
		assert.Equal(networkCount, len(networks))

		// Assert the server knows about the networks added
		resp, err := client.NetworkRange(context.TODO(), &api.NetworkRangeRequest{})
		assert.NoError(err)
		assert.Equal(networkCount, len(resp.Networks))

		for k, v := range networks {
			resp, err = client.NetworkRange(context.TODO(), &api.NetworkRangeRequest{ID: k})
			assert.NoError(err)
			assert.Equal(1, len(resp.Networks))
			assert.Equal(v.ID, resp.Networks[0].ID)
			assert.Equal(v.Cidr, resp.Networks[0].Cidr)
			assert.Equal(v.Annotations, resp.Networks[0].Annotations)
		}

		// Assert errors return when bogus data is used
		resp, err = client.NetworkRange(context.TODO(), &api.NetworkRangeRequest{ID: "foobar"})
		assert.Error(err)

	})

	test.execute(t)
}

func TestSrvPool(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {

	})

	test.execute(t)
}
