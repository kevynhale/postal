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
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/server"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// serverCmd represents the bind command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start postal server",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return errors.Wrap(err, "failed to open etcd client")
		}
		defer cli.Close()

		lis, err := net.Listen("tcp", endpoint)
		if err != nil {
			return errors.Wrapf(err, "failed to listen on endpoint %s", endpoint)
		}
		defer lis.Close()

		grpcServer := grpc.NewServer()
		srv := server.NewServer(cli)
		srv.Register(grpcServer)
		return grpcServer.Serve(lis)
	},
}

func init() {
	PostalCmd.AddCommand(serverCmd)
}
