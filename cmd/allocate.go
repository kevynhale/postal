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

package cmd

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/jive/postal/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var allocateBulk bool

// allocateCmd represents the allocate command
var allocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "allocate an address to a pool",
	Long:  `postal allocate <networkID> <poolID> (<optional_address>)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if allocateBulk {
			return allocateBulkFn(cmd, args)
		}
		return allocateSingleFn(cmd, args)

	},
}

func allocateSingleFn(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("invalid arguments")
	}

	req := &api.AllocateAddressRequest{
		PoolID: &api.Pool_PoolID{
			NetworkID: args[0],
			ID:        args[1],
		},
	}

	if len(args) == 3 {
		req.Address = args[2]
	}

	resp, err := mustClientFromCmd(cmd).AllocateAddress(context.TODO(), req)
	if err != nil {
		return errors.Wrap(err, "allocate rpc failed")
	}

	display.AllocateAddress(resp)

	return nil
}

func allocateBulkFn(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("invalid arguments")
	}

	req := &api.BulkAllocateAddressRequest{
		PoolID: &api.Pool_PoolID{
			NetworkID: args[0],
			ID:        args[1],
		},
		Cidr: args[2],
	}

	resp, err := mustClientFromCmd(cmd).BulkAllocateAddress(context.TODO(), req)
	if err != nil {
		return errors.Wrap(err, "bulk allocate rpc failed")
	}

	display.BulkAllocateAddress(resp)
	/*
		_, ipnet, _ := net.ParseCIDR(args[2])
		ones, bits := ipnet.Mask.Size()

		if len(resp.Errors) > 0 {
			fmt.Printf("%d of %v addresses successfully allocated to pool.\n", len(resp.Bindings), math.Pow(float64(2), float64(bits-ones)))
			fmt.Println("The following addresses failed to allocate:")
			for ip, berr := range resp.Errors {
				fmt.Printf("---> %s: %s\n", ip, berr.Message)
			}
		} else {
			fmt.Printf("All %v addresses successfully allocated to pool.\n", math.Pow(float64(2), float64(bits-ones)))
		}*/
	return nil
}

func init() {
	PostalCmd.AddCommand(allocateCmd)

	allocateCmd.Flags().BoolVarP(&allocateBulk, "bulk", "b", false, "use cidr block instead of an IP")
}
