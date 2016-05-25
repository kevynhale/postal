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

// bindCmd represents the bind command
var bindCmd = &cobra.Command{
	Use:   "bind",
	Short: "bind an address in a pool",
	Long:  `postal bind <networkID> <poolID> (<optional_address>)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("invalid arguments")
		}

		annotationsVars, err := cmd.Flags().GetStringSlice("annotations")
		if err != nil {
			return err
		}
		annotations := parseAnnotations(annotationsVars)

		req := &api.BindAddressRequest{
			PoolID: &api.Pool_PoolID{
				NetworkID: args[0],
				ID:        args[1],
			},
			Annotations: annotations,
		}

		if len(args) == 3 {
			req.Address = args[2]
		}

		resp, err := mustClientFromCmd(cmd).BindAddress(context.TODO(), req)
		if err != nil {
			return errors.Wrap(err, "bind rpc failed")
		}

		display.BindAddress(resp)

		return nil
	},
}

func init() {
	PostalCmd.AddCommand(bindCmd)
}
