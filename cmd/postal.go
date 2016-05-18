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
	"os"

	"google.golang.org/grpc"

	"github.com/coreos/pkg/capnslog"
	"github.com/jive/postal/api"
	"github.com/spf13/cobra"
)

var plog = capnslog.NewPackageLogger("github.com/jive/postal", "cmd")

var cfgFile string
var endpoint string
var insecure bool

var client api.PostalClient

// PostalCmd represents the base command when called without any subcommands
var PostalCmd = &cobra.Command{
	Use:   "postal",
	Short: "CLI tool to manage postal service",
	Long:  ``,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := PostalCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initClient)

	PostalCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.netctl.yaml)")
	PostalCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "localhost:7542", "postal server endpoint")
	PostalCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "enable insecure communication")

}

func initClient() {
	if os.Args[1] == "server" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		return
	}
	ops := []grpc.DialOption{}
	if insecure {
		ops = append(ops, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(endpoint, ops...)
	if err != nil {
		fmt.Println("Could not connect to endpoint: ", endpoint)
		//os.Exit(2)
	}
	client = api.NewPostalClient(conn)
}
