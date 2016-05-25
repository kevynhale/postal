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
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
)

var plog = capnslog.NewPackageLogger("github.com/jive/postal", "cmd")
var (
	globalFlags           = GlobalFlags{}
	defaultDialTimeout    = 2 * time.Second
	defaultCommandTimeOut = 10 * time.Second
)

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
	PostalCmd.PersistentFlags().StringVar(&globalFlags.Endpoint, "endpoint", "127.0.0.1:7542", "gRPC endpoints")

	PostalCmd.PersistentFlags().StringVarP(&globalFlags.OutputFormat, "write-out", "w", "simple", "set the output format (simple, json, table)")

	PostalCmd.PersistentFlags().DurationVar(&globalFlags.DialTimeout, "dial-timeout", defaultDialTimeout, "dial timeout for client connections")
	PostalCmd.PersistentFlags().DurationVar(&globalFlags.CommandTimeOut, "command-timeout", defaultCommandTimeOut, "timeout for short running command (excluding dial timeout)")

	PostalCmd.PersistentFlags().BoolVar(&globalFlags.Insecure, "insecure-transport", true, "disable transport security for client connections")
	PostalCmd.PersistentFlags().BoolVar(&globalFlags.InsecureSkipVerify, "insecure-skip-tls-verify", false, "skip server certificate verification")
	PostalCmd.PersistentFlags().StringVar(&globalFlags.CertFile, "cert", "", "identify secure client using this TLS certificate file")
	PostalCmd.PersistentFlags().StringVar(&globalFlags.KeyFile, "key", "", "identify secure client using this TLS key file")
	PostalCmd.PersistentFlags().StringVar(&globalFlags.CAFile, "cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle")

}
