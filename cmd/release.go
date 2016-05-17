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

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "release an address in a pool",
	Long:  `postal release <networkID> (<poolID> <bindingID>|<address>)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hard, err := cmd.Flags().GetBool("hard")
		if err != nil {
			return errors.Wrap(err, "failed to parse --hard flag")
		}

		req := &api.ReleaseAddressRequest{
			PoolID: &api.Pool_PoolID{},
			Hard:   hard,
		}

		switch len(args) {
		case 2:
			req.PoolID.NetworkID = args[0]
			req.Address = args[1]
		case 3:
			req.PoolID.NetworkID = args[0]
			req.PoolID.ID = args[1]
			req.BindingID = args[2]
		default:
			return fmt.Errorf("invalid arguments")
		}

		_, err = client.ReleaseAddress(context.TODO(), req)
		if err != nil {
			return errors.Wrap(err, "bind rpc failed")
		}

		fmt.Println("Released")

		return nil
	},
}

func init() {
	PostalCmd.AddCommand(releaseCmd)

	releaseCmd.Flags().Bool("hard", false, "hard release")
}
