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
	"net"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/jive/postal/api"
	"github.com/jive/postal/cmd/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "create a resource",
	Long:  ``,
}

// networkCmd represents the network command
var createNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "create a network",
	Long: `You must specify a block of addresses in CIDR format as the first argument
to this command. You may subsequently add metadata via annotations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("invalid arguments")
		}
		annotationsVars, err := cmd.Flags().GetStringSlice("annotation")
		if err != nil {
			return err
		}
		annotations := map[string]string{}
		for idx := range annotationsVars {
			split := strings.Split(annotationsVars[idx], "=")
			annotations[split[0]] = split[1]
		}

		_, cidr, err := net.ParseCIDR(args[0])
		if err != nil {
			return errors.Wrap(err, "failed to parse cidr")
		}

		resp, err := client.NetworkAdd(context.TODO(), &api.NetworkAddRequest{
			Annotations: annotations,
			Cidr:        cidr.String(),
		})

		if err != nil {
			return err
		}

		util.PrintNetwork(resp.Network)
		return nil
	},
}

var createPoolCmd = &cobra.Command{
	Use:   "pool",
	Short: "create a pool from a network",
	Long:  `A pool is a set of `,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return errors.New("<networkID> <max> must be the only 2 arguments")
		}

		networkID := args[0]

		max, err := strconv.ParseUint(args[1], 0, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse max argument")
		}

		annotationsVars, err := cmd.Flags().GetStringSlice("annotation")
		if err != nil {
			return err
		}
		annotations := util.ParseAnnotations(annotationsVars)

		poolTypeStr, err := cmd.Flags().GetString("type")
		if err != nil {
			return err
		}

		var poolType api.Pool_Type
		if poolTypeStr == "dynamic" {
			poolType = api.Pool_DYNAMIC
		}
		if poolTypeStr == "fixed" {
			poolType = api.Pool_FIXED
		}

		resp, err := client.PoolAdd(context.TODO(), &api.PoolAddRequest{
			NetworkID:   networkID,
			Annotations: annotations,
			Maximum:     max,
			Type:        poolType,
		})
		if err != nil {
			return err
		}

		util.PrintPool(resp.Pool)

		return nil
	},
}

func init() {
	PostalCmd.AddCommand(createCmd)
	createCmd.AddCommand(createPoolCmd)
	createCmd.AddCommand(createNetworkCmd)

	createNetworkCmd.Flags().StringSliceP("annotation", "a", []string{}, "key=value pair of data to annotate the network with")

	createPoolCmd.Flags().StringSliceP("annotation", "a", []string{}, "key=value pair of data to annotate the pool with")
	createPoolCmd.Flags().StringP("type", "t", "dynamic", "pool type")
}
