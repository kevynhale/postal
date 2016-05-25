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
	"strconv"

	"github.com/jive/postal/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// set-maxCmd represents the set-max command
var setmaxCmd = &cobra.Command{
	Use:   "set-max <networkID> <poolID> <max>",
	Short: "set the maximum address limit for a pool",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 3 {
			return errors.New("<networkID> <poolID> <max> must be the only 3 arguments")
		}

		networkID := args[0]
		poolID := args[1]
		max, err := strconv.ParseUint(args[2], 0, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse max argument")
		}

		resp, err := mustClientFromCmd(cmd).PoolSetMax(context.TODO(), &api.PoolSetMaxRequest{
			PoolID: &api.Pool_PoolID{
				NetworkID: networkID,
				ID:        poolID,
			},
			Maximum: max,
		})
		if err != nil {
			return err
		}

		display.PoolSetMax(resp)

		return nil
	},
}

func init() {
	PostalCmd.AddCommand(setmaxCmd)

}
