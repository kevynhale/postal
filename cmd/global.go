package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/coreos/etcd/pkg/flags"
	"github.com/jive/postal/api"
	"github.com/spf13/cobra"
)

// GlobalFlags are flags that defined globally
// and are inherited to all sub-commands.
type GlobalFlags struct {
	Insecure           bool
	InsecureSkipVerify bool
	Endpoint           string
	DialTimeout        time.Duration
	CommandTimeOut     time.Duration

	CertFile string
	KeyFile  string
	CAFile   string

	OutputFormat string
}

type secureCfg struct {
	cert   string
	key    string
	cacert string

	insecureTransport  bool
	insecureSkipVerify bool
}

var display printer = &simplePrinter{}

func mustClientFromCmd(cmd *cobra.Command) api.PostalClient {
	flags.SetPflagsFromEnv("POSTAL", cmd.InheritedFlags())

	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	dialTimeout := dialTimeoutFromCmd(cmd)
	sec := secureCfgFromCmd(cmd)

	initDisplayFromCmd(cmd)

	return mustClient(endpoint, dialTimeout, sec)
}

func mustClient(endpoint string, dialTimeout time.Duration, scfg *secureCfg) api.PostalClient {
	ops := []grpc.DialOption{grpc.WithTimeout(dialTimeout)}
	if scfg.insecureTransport {
		ops = append(ops, grpc.WithInsecure())
	} else {

		ops = append(ops, grpc.WithTransportCredentials(credentials.NewTLS(
			mustBuildTLSConfig(scfg),
		)))
	}
	conn, err := grpc.Dial(endpoint, ops...)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}
	return api.NewPostalClient(conn)
}

func initDisplayFromCmd(cmd *cobra.Command) {
	outputType, err := cmd.Flags().GetString("write-out")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if display = NewPrinter(outputType); display == nil {
		ExitWithError(ExitBadFeature, errors.New("unsupported output format"))
	}
}

func dialTimeoutFromCmd(cmd *cobra.Command) time.Duration {
	dialTimeout, err := cmd.Flags().GetDuration("dial-timeout")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return dialTimeout
}

func secureCfgFromCmd(cmd *cobra.Command) *secureCfg {
	cert, key, cacert := keyAndCertFromCmd(cmd)
	insecureTr := insecureTransportFromCmd(cmd)
	skipVerify := insecureSkipVerifyFromCmd(cmd)

	return &secureCfg{
		cert:   cert,
		key:    key,
		cacert: cacert,

		insecureTransport:  insecureTr,
		insecureSkipVerify: skipVerify,
	}
}

func insecureTransportFromCmd(cmd *cobra.Command) bool {
	insecureTr, err := cmd.Flags().GetBool("insecure-transport")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return insecureTr
}

func insecureSkipVerifyFromCmd(cmd *cobra.Command) bool {
	skipVerify, err := cmd.Flags().GetBool("insecure-skip-tls-verify")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return skipVerify
}

func keyAndCertFromCmd(cmd *cobra.Command) (cert, key, cacert string) {
	var err error
	if cert, err = cmd.Flags().GetString("cert"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if cert == "" && cmd.Flags().Changed("cert") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --cert option"))
	}

	if key, err = cmd.Flags().GetString("key"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if key == "" && cmd.Flags().Changed("key") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --key option"))
	}

	if cacert, err = cmd.Flags().GetString("cacert"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if cacert == "" && cmd.Flags().Changed("cacert") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --cacert option"))
	}

	return cert, key, cacert
}

func mustBuildTLSConfig(scfg *secureCfg) *tls.Config {
	tlsConfig := &tls.Config{}
	if scfg.cacert != "" {
		b, err := ioutil.ReadFile(scfg.cacert)
		if err != nil {
			ExitWithError(ExitBadArgs, err)
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			ExitWithError(ExitBadArgs, fmt.Errorf("tls: failed to append root certificates"))
		}
		tlsConfig.RootCAs = cp
	}

	if scfg.cert != "" && scfg.key != "" {
		clientCert, err := tls.LoadX509KeyPair(scfg.cert, scfg.key)
		if err != nil {
			ExitWithError(ExitBadArgs, err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	tlsConfig.InsecureSkipVerify = scfg.insecureSkipVerify

	return tlsConfig
}
