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
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/pkg/capnslog"
	"github.com/jive/postal/postal"
	"github.com/jive/postal/server"
	"github.com/soheilhy/cmux"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var etcdEndpoints []string
var etcdDialTimeout time.Duration
var serverDebug bool
var enableHTTP bool

// serverCmd represents the bind command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start postal server",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if serverDebug {
			capnslog.SetGlobalLogLevel(capnslog.DEBUG)
		}
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

		plog.Info("starting binding janitor")
		go postal.NewJanitor(cli).Run()

		var lis net.Listener
		plog.Infof("listening for client connections on [%s]", globalFlags.Endpoint)
		lis, err = net.Listen("tcp", globalFlags.Endpoint)
		if err != nil {
			plog.Fatalf("failed to start listener: %s", err)
		}
		defer lis.Close()

		scfg := secureCfgFromCmd(cmd)
		if scfg.insecureTransport {
			plog.Info("listener configured for insecure transport")
		} else {
			plog.Info("configuring listener for TLS transport")
			lis = tls.NewListener(lis, mustBuildTLSConfig(scfg))
		}

		m := cmux.New(lis)
		grpcL := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
		httpL := m.Match(cmux.HTTP1Fast())

		grpcServer := grpc.NewServer()
		srv := server.NewServer(cli)
		srv.Register(grpcServer)
		go grpcServer.Serve(grpcL)

		if enableHTTP {
			httpServer := &http.Server{
				Handler: srv,
			}
			go httpServer.Serve(httpL)
		}

		m.Serve()
	},
}

func init() {
	PostalCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringSliceVar(&etcdEndpoints, "etcd", []string{"127.0.0.1:2379"}, "etcd servers to use")
	serverCmd.Flags().DurationVar(&etcdDialTimeout, "etcd-timeout", 5*time.Second, "etcd dial timeout")
	serverCmd.Flags().BoolVar(&serverDebug, "debug", false, "enable debug logging")
	serverCmd.Flags().BoolVar(&enableHTTP, "http", false, "enable http api")
}
