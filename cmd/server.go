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
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var etcdEndpoints []string
var etcdDialTimeout time.Duration

// serverCmd represents the bind command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start postal server",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		plog.Info("starting postal server")
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   etcdEndpoints,
			DialTimeout: etcdDialTimeout,
		})

		plog.Infof("configuring server with etcd endpoints [%s]", cli.Endpoints())
		if err != nil {
			plog.Fatalf("failed to open etcd client conn: %s", err)
		}
		defer cli.Close()

		plog.Infof("listening for client connections on [%s]", endpoint)
		lis, err := net.Listen("tcp", endpoint)
		if err != nil {
			plog.Fatalf("failed to start listener: %s", err)
		}
		defer lis.Close()

		grpcServer := grpc.NewServer()
		srv := server.NewServer(cli)
		srv.Register(grpcServer)
		grpcServer.Serve(lis)
	},
}

func init() {
	PostalCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringSliceVar(&etcdEndpoints, "etcd", []string{"127.0.0.1:2379"}, "etcd servers to use")
	serverCmd.Flags().DurationVar(&etcdDialTimeout, "etcd-timeout", 5*time.Second, "etcd dial timeout")
}
