package server

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/postal"
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

	go postal.NewJanitor(cli).Run()

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
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        "10.0.0.0/16",
		})
		assert.NoError(networkErr)

		poolCount := 5
		pools := map[string]*api.Pool{}
		poolMax := uint64(10)
		for i := 0; i < poolCount; i++ {
			resp, err := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
				NetworkID:   networkResp.Network.ID,
				Annotations: map[string]string{},
				Maximum:     poolMax,
				Type:        api.Pool_DYNAMIC,
			})

			assert.NoError(err)
			assert.Equal(networkResp.Network.ID, resp.Pool.ID.NetworkID)
			assert.Equal(poolMax, resp.Pool.MaximumAddresses)
			assert.Equal(api.Pool_DYNAMIC, resp.Pool.Type)
			pools[resp.Pool.ID.ID] = resp.Pool
		}
		assert.Equal(poolCount, len(pools))

		resp, err := client.PoolRange(context.TODO(), &api.PoolRangeRequest{
			ID: &api.Pool_PoolID{
				NetworkID: networkResp.Network.ID,
			},
		})
		assert.NoError(err)
		assert.Equal(int32(poolCount), resp.Size_)
		assert.Equal(poolCount, len(resp.Pools))

		for k, v := range pools {
			resp, err = client.PoolRange(context.TODO(), &api.PoolRangeRequest{
				ID: &api.Pool_PoolID{
					NetworkID: networkResp.Network.ID,
					ID:        k,
				},
			})
			assert.NoError(err)
			assert.Equal(int32(1), resp.Size_)
			assert.Equal(1, len(resp.Pools))
			assert.Equal(v.ID.NetworkID, resp.Pools[0].ID.NetworkID)
			assert.Equal(v.ID.ID, resp.Pools[0].ID.ID)
			assert.Equal(v.MaximumAddresses, resp.Pools[0].MaximumAddresses)
			assert.Equal(v.Type, resp.Pools[0].Type)
		}

		resp, err = client.PoolRange(context.TODO(), &api.PoolRangeRequest{
			ID: &api.Pool_PoolID{
				NetworkID: networkResp.Network.ID,
				ID:        "foo",
			},
		})
		assert.Error(err)

		resp, err = client.PoolRange(context.TODO(), &api.PoolRangeRequest{
			ID: &api.Pool_PoolID{
				NetworkID: "foo",
			},
		})
		assert.Error(err)
	})
	test.execute(t)
}

func TestSrvDynamicPool(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {
		_, networkCidr, _ := net.ParseCIDR("10.0.0.0/16")
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        networkCidr.String(),
		})
		assert.NoError(networkErr)

		poolResp, poolErr := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     3,
			Type:        api.Pool_DYNAMIC,
		})
		assert.NoError(poolErr)

		// Allocated: 1
		// Bound:     0
		allocResp, allocErr := client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(allocErr)
		allocatedAddr := net.ParseIP(allocResp.Binding.Address)
		assert.False(allocatedAddr.IsUnspecified())
		assert.True(networkCidr.Contains(allocatedAddr))

		// Allocated: 0
		// Bound:     1
		bindResp, bindErr := client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)
		assert.Equal(allocResp.Binding.AllocateTime, bindResp.Binding.AllocateTime)
		boundAddr := net.ParseIP(bindResp.Binding.Address)
		assert.Equal(allocatedAddr, boundAddr)

		// Allocated: 0
		// Bound:     1
		// Attempting to bind already bound address, should error
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: boundAddr.String(),
		})
		assert.Error(bindErr)

		// Allocated: 0
		// Bound:     2
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)

		// Allocated: 0
		// Bound:     3
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)

		// Allocated: 0
		// Bound:     4 ** over maximum, should error
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.Error(bindErr)

		bindingRngResp, err := client.BindingRange(context.TODO(), &api.BindingRangeRequest{
			NetworkID: networkResp.Network.ID,
			Filters:   map[string]string{"_address": allocatedAddr.String()},
		})
		assert.NoError(err)
		assert.Len(bindingRngResp.Bindings, 1)

		// Allocated: 1
		// Bound:     2
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: bindingRngResp.Bindings[0].ID,
		})
		assert.NoError(err)

		// Allocated: 1
		// Bound:     2
		// Attempting to release non bound address should error
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: bindingRngResp.Bindings[0].ID,
		})
		assert.Error(err)

		// Allocated: 0
		// Bound:     3
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)
		boundAddr = net.ParseIP(bindResp.Binding.Address)
		assert.Equal(allocatedAddr, boundAddr)

		// Allocated: 0
		// Bound:     2
		// Hard release expires the binding immediately
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: bindingRngResp.Bindings[0].ID,
			Hard:      true,
		})
		assert.NoError(err)

		// Allocated: 0
		// Bound:     3
		// Should be a different address than the previously allocated one
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)
		boundAddr = net.ParseIP(bindResp.Binding.Address)
		assert.NotEqual(allocatedAddr, boundAddr)

	})

	test.execute(t)
}

func TestSrvFixedPool(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {
		_, networkCidr, _ := net.ParseCIDR("10.0.0.0/16")
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        networkCidr.String(),
		})
		assert.NoError(networkErr)

		poolResp, poolErr := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     3,
			Type:        api.Pool_FIXED,
		})
		assert.NoError(poolErr)

		// Allocated: 1
		// Bound:     0
		allocResp, allocErr := client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(allocErr)
		allocatedAddr := net.ParseIP(allocResp.Binding.Address)
		allocatedBinding := allocResp.Binding.ID
		assert.False(allocatedAddr.IsUnspecified())
		assert.True(networkCidr.Contains(allocatedAddr))

		// Allocated: 0
		// Bound:     1
		bindResp, bindErr := client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)
		assert.Equal(allocResp.Binding.AllocateTime, bindResp.Binding.AllocateTime)
		boundAddr := net.ParseIP(bindResp.Binding.Address)
		assert.Equal(allocatedAddr, boundAddr)

		// Allocated: 0
		// Bound:     1
		// Attempting to bind already bound address should error
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: boundAddr.String(),
		})
		assert.Error(bindErr)

		// Allocated: 0
		// Bound:     1
		// Attempting to bind any address with none allocated should error
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.Error(bindErr)

		// Allocated: 1
		// Bound:     1
		allocResp, allocErr = client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(allocErr)

		// Allocated: 2
		// Bound:     1
		allocResp, allocErr = client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(allocErr)

		// Allocated: 1
		// Bound:     2
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)

		// Allocated: 2 ** over maximum should error
		// Bound:     2
		allocResp, allocErr = client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.Error(allocErr)

		bindingRngResp, err := client.BindingRange(context.TODO(), &api.BindingRangeRequest{
			NetworkID: poolResp.Pool.ID.NetworkID,
			Filters:   map[string]string{"_id": allocatedBinding},
		})
		assert.NoError(err)
		assert.Len(bindingRngResp.Bindings, 1)

		// Allocated: 0
		// Bound:     3
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)

		// Allocated: 1
		// Bound:     2
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: bindingRngResp.Bindings[0].ID,
		})
		assert.NoError(err)

		// Allocated: 1
		// Bound:     2
		// Attempting to release non bound address should error
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			BindingID: bindingRngResp.Bindings[0].ID,
		})
		assert.Error(err)

		// Allocated: 0
		// Bound:     3
		bindResp, bindErr = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(bindErr)
		boundAddr = net.ParseIP(bindResp.Binding.Address)
		assert.Equal(allocatedAddr, boundAddr)

		// Allocated: 0
		// Bound:     2
		// Hard release expires the binding immediately
		_, err = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: bindingRngResp.Bindings[0].ID,
			Hard:      true,
		})
		assert.NoError(err)

		// Allocated: 0
		// Bound:     2
		// Since we previouslly force released, there should be no allocated address to bind.
		_, err = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.Error(err)

		// Allocated: 1
		// Bound:     2
		allocResp, allocErr = client.AllocateAddress(context.TODO(), &api.AllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.NoError(allocErr)
		newAllocatedAddr := net.ParseIP(allocResp.Binding.Address)
		assert.NotEqual(allocatedAddr, newAllocatedAddr)
	})

	test.execute(t)
}

func TestSrvBulkAllocate(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {
		_, networkCidr, _ := net.ParseCIDR("10.0.0.0/16")
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        networkCidr.String(),
		})
		assert.NoError(networkErr)

		poolResp, poolErr := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     10000,
			Type:        api.Pool_DYNAMIC,
		})
		assert.NoError(poolErr)

		allocResp, allocErr := client.BulkAllocateAddress(context.TODO(), &api.BulkAllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
			Cidr:   "10.0.255.0/24",
		})
		assert.NoError(allocErr)
		assert.Equal(1, len(allocResp.GetErrors()))
		assert.Equal(255, len(allocResp.GetBindings()))

		_, allocErr = client.BulkAllocateAddress(context.TODO(), &api.BulkAllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
		})
		assert.Error(allocErr)

		allocResp, allocErr = client.BulkAllocateAddress(context.TODO(), &api.BulkAllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
			Cidr:   "10.0.254.0/23",
		})
		assert.NoError(allocErr)
		assert.Equal(256, len(allocResp.GetErrors()))
		assert.Equal(256, len(allocResp.GetBindings()))

		allocResp, allocErr = client.BulkAllocateAddress(context.TODO(), &api.BulkAllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
			Cidr:   "10.0.0.0/26",
		})
		assert.NoError(allocErr)
		assert.Equal(1, len(allocResp.GetErrors()))
		assert.Equal(63, len(allocResp.GetBindings()))

	})

	test.execute(t)
}

func TestAllocateReleaseAllocate(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient) {
		_, networkCidr, _ := net.ParseCIDR("10.0.0.0/16")
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        networkCidr.String(),
		})
		assert.NoError(networkErr)

		poolResp, poolErr := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     10000,
			Type:        api.Pool_DYNAMIC,
		})
		assert.NoError(poolErr)

		resp1, err1 := client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		resp2, err2 := client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		resp3, err3 := client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		assert.NoError(err1)
		assert.NoError(err2)
		assert.NoError(err3)
		assert.NotEqual(resp1.Binding.Address, resp2.Binding.Address)
		assert.NotEqual(resp1.Binding.Address, resp3.Binding.Address)

		_, err1 = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: resp1.Binding.ID,
		})
		assert.NoError(err1)

		_, err2 = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: resp2.Binding.ID,
		})
		assert.NoError(err2)

		_, err3 = client.ReleaseAddress(context.TODO(), &api.ReleaseAddressRequest{
			PoolID:    poolResp.Pool.ID,
			BindingID: resp3.Binding.ID,
		})
		assert.NoError(err3)

		resp1, err1 = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		resp2, err2 = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		resp3, err3 = client.BindAddress(context.TODO(), &api.BindAddressRequest{
			PoolID:  poolResp.Pool.ID,
			Address: "",
		})

		assert.NoError(err1)
		assert.NoError(err2)
		assert.NoError(err3)
		assert.NotEqual(resp1.Binding.Address, resp2.Binding.Address)
		assert.NotEqual(resp1.Binding.Address, resp3.Binding.Address)
	})

	test.execute(t)
}
