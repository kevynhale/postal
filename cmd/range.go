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
	"strings"

	"golang.org/x/net/context"

	"github.com/jive/postal/api"
	"github.com/jive/postal/cmd/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var human bool

// rangeCmd represents the range command
var rangeCmd = &cobra.Command{
	Use:   "range",
	Short: "inspect resources",
	Long:  `The range command allows you to inspect sets of postal resources.`,
}

var networksCmd = &cobra.Command{
	Use:   "networks",
	Short: "view networks",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		var req *api.NetworkRangeRequest
		if len(args) == 0 {
			req = &api.NetworkRangeRequest{}
		} else if len(strings.Split(args[0], "=")) == 1 {
			req = &api.NetworkRangeRequest{ID: args[0]}
		} else {
			req = &api.NetworkRangeRequest{Filters: map[string]string{}}
			for idx := range args {
				vals := strings.Split(args[idx], "=")
				if len(vals) == 2 {
					req.Filters[vals[0]] = vals[1]
				}
			}
		}

		resp, err := client.NetworkRange(context.TODO(), req)
		if err != nil {
			return errors.Wrap(err, "failed to complete network range request")
		}
		util.PrintNetworks(resp.Networks)
		return nil
	},
}

var poolsCmd = &cobra.Command{
	Use:   "pools",
	Short: "view pools",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &api.PoolRangeRequest{ID: &api.Pool_PoolID{}}
		if len(args) == 0 {
			return fmt.Errorf("first argument must be a network ID")
		}

		req.ID.NetworkID = args[0]

		if len(args) > 1 {
			if len(strings.Split(args[1], "=")) == 1 {
				req.ID.ID = args[1]
			} else {
				req.Filters = util.ParseAnnotations(args[1:len(args)])
			}
		}

		resp, err := client.PoolRange(context.TODO(), req)
		if err != nil {
			return errors.Wrap(err, "failed to complete pool range request")
		}

		util.PrintPools(resp.Pools)
		return nil
	},
}

var bindingsCmd = &cobra.Command{
	Use:   "bindings",
	Short: "view bindings",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &api.BindingRangeRequest{}
		if len(args) == 0 {
			return fmt.Errorf("first argument must be a network ID")
		}
		req.NetworkID = args[0]
		req.Filters = util.ParseAnnotations(args[1:len(args)])

		resp, err := client.BindingRange(context.TODO(), req)
		if err != nil {
			return errors.Wrap(err, "failed to complete binding range request")
		}

		util.PrintBindings(resp.Bindings, human)
		return nil
	},
}

func init() {
	PostalCmd.AddCommand(rangeCmd)

	rangeCmd.AddCommand(networksCmd)
	rangeCmd.AddCommand(poolsCmd)
	rangeCmd.AddCommand(bindingsCmd)

	bindingsCmd.Flags().BoolVarP(&human, "human", "d", false, "humanize output")

}
