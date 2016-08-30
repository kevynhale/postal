package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"github.com/jive/postal/api"
	"github.com/stretchr/testify/assert"
)

func TestHTTPNetworkRange(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
		// Add some networks to the server
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

		resp, err := httpClient.Get(endpoint + "/v1/networks")
		assert.NoError(err)
		if resp.StatusCode != http.StatusOK {
			buf := bytes.Buffer{}
			buf.ReadFrom(resp.Body)
			fmt.Println("Expected 200 OK, got", resp.Status)
			fmt.Println(buf.String())
			t.FailNow()
		}

		rangeResp := &api.NetworkRangeResponse{}
		err = json.NewDecoder(resp.Body).Decode(rangeResp)
		assert.NoError(err)

		assert.Equal(networkCount, len(rangeResp.Networks))
	})

	test.execute(t)
}

func TestHTTPNetworkRangeID(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
		// Add some networks to the server
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

		for id := range networks {
			httpReq, err := http.NewRequest("GET", (endpoint + "/v1/networks/" + id), nil)
			assert.NoError(err)
			resp, err := httpClient.Do(httpReq)
			assert.NoError(err)
			if resp.StatusCode != http.StatusOK {
				buf := bytes.Buffer{}
				buf.ReadFrom(resp.Body)
				fmt.Println("Expected 200 OK, got", resp.Status)
				fmt.Println(buf.String())
				t.FailNow()
			}

			rangeResp := &api.NetworkRangeResponse{}
			err = json.NewDecoder(resp.Body).Decode(rangeResp)
			assert.NoError(err)

			assert.Equal(1, len(rangeResp.Networks))

			assert.EqualValues(*networks[id], *rangeResp.Networks[0])
		}

	})

	test.execute(t)
}

func TestHTTPPoolRange(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
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

		httpReq, err := http.NewRequest("GET", (endpoint + "/v1/networks/" + networkResp.Network.ID + "/pools"), nil)
		assert.NoError(err)
		resp, err := httpClient.Do(httpReq)
		assert.NoError(err)
		if resp.StatusCode != http.StatusOK {
			buf := bytes.Buffer{}
			buf.ReadFrom(resp.Body)
			fmt.Println("Expected 200 OK, got", resp.Status)
			fmt.Println(buf.String())
			t.FailNow()
		}

		rangeResp := &api.PoolRangeResponse{}
		err = json.NewDecoder(resp.Body).Decode(rangeResp)
		assert.NoError(err)

		assert.NoError(err)
		assert.Equal(int32(poolCount), rangeResp.Size_)
		assert.Equal(poolCount, len(rangeResp.Pools))
	})
	test.execute(t)
}

func TestHTTPNetworkBindingsRange(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        "10.0.0.0/16",
		})
		assert.NoError(networkErr)
		poolResp, err := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     1000,
			Type:        api.Pool_FIXED,
		})

		allocResp, allocErr := client.BulkAllocateAddress(context.TODO(), &api.BulkAllocateAddressRequest{
			PoolID: poolResp.Pool.ID,
			Cidr:   "10.0.40.0/24",
		})
		assert.NoError(allocErr)
		assert.Equal(0, len(allocResp.GetErrors()))
		assert.Equal(256, len(allocResp.GetBindings()))

		httpReq, err := http.NewRequest("GET", (endpoint + "/v1/networks/" + networkResp.Network.ID + "/bindings"), nil)
		assert.NoError(err)
		resp, err := httpClient.Do(httpReq)
		assert.NoError(err)
		if resp.StatusCode != http.StatusOK {
			buf := bytes.Buffer{}
			buf.ReadFrom(resp.Body)
			fmt.Println("Expected 200 OK, got", resp.Status)
			fmt.Println(buf.String())
			t.FailNow()
		}

		rangeResp := &api.BindingRangeResponse{}
		err = json.NewDecoder(resp.Body).Decode(rangeResp)
		assert.NoError(err)
		assert.Equal(int32(256), rangeResp.Size_)
	})
	test.execute(t)
}

func TestHTTPAllocate(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        "10.0.0.0/16",
		})
		assert.NoError(networkErr)
		poolResp, err := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     1000,
			Type:        api.Pool_FIXED,
		})

		httpReq, err := http.NewRequest("POST", (endpoint + "/v1/networks/" + networkResp.Network.ID + "/pools/" + poolResp.Pool.ID.ID + "/_allocate"), nil)
		assert.NoError(err)
		resp, err := httpClient.Do(httpReq)
		assert.NoError(err)
		if resp.StatusCode != http.StatusOK {
			buf := bytes.Buffer{}
			buf.ReadFrom(resp.Body)
			fmt.Println("Expected 200 OK, got", resp.Status)
			fmt.Println(buf.String())
			t.FailNow()
		}

		allocResp := &api.AllocateAddressResponse{}
		err = json.NewDecoder(resp.Body).Decode(allocResp)
		assert.NoError(err)

		assert.NotEmpty(allocResp.Binding.Address)
	})
	test.execute(t)
}

func TestHTTPBind(t *testing.T) {
	test := sandboxedServerTest(func(assert *assert.Assertions, client api.PostalClient, httpClient *http.Client, endpoint string) {
		networkResp, networkErr := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: map[string]string{},
			Cidr:        "10.0.0.0/16",
		})
		assert.NoError(networkErr)
		poolResp, err := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkResp.Network.ID,
			Annotations: map[string]string{},
			Maximum:     1000,
			Type:        api.Pool_DYNAMIC,
		})

		httpReq, err := http.NewRequest("POST", (endpoint + "/v1/networks/" + networkResp.Network.ID + "/pools/" + poolResp.Pool.ID.ID + "/_bind"), nil)
		assert.NoError(err)
		resp, err := httpClient.Do(httpReq)
		assert.NoError(err)
		if resp.StatusCode != http.StatusOK {
			buf := bytes.Buffer{}
			buf.ReadFrom(resp.Body)
			fmt.Println("Expected 200 OK, got", resp.Status)
			fmt.Println(buf.String())
			t.FailNow()
		}

		bindResp := &api.BindAddressResponse{}
		err = json.NewDecoder(resp.Body).Decode(bindResp)
		assert.NoError(err)

		assert.NotEmpty(bindResp.Binding.Address)
	})
	test.execute(t)
}
