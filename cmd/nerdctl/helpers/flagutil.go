/*
   Copyright The containerd Authors.

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

package helpers

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/containerd/nerdctl/v2/pkg"
	"github.com/containerd/nerdctl/v2/pkg/api/types"
)

func VerifyOptions(cmd *cobra.Command) (opt types.ImageVerifyOptions, err error) {
	if opt.Provider, err = cmd.Flags().GetString("verify"); err != nil {
		return
	}
	if opt.CosignKey, err = cmd.Flags().GetString("cosign-key"); err != nil {
		return
	}
	if opt.CosignCertificateIdentity, err = cmd.Flags().GetString("cosign-certificate-identity"); err != nil {
		return
	}
	if opt.CosignCertificateIdentityRegexp, err = cmd.Flags().GetString("cosign-certificate-identity-regexp"); err != nil {
		return
	}
	if opt.CosignCertificateOidcIssuer, err = cmd.Flags().GetString("cosign-certificate-oidc-issuer"); err != nil {
		return
	}
	if opt.CosignCertificateOidcIssuerRegexp, err = cmd.Flags().GetString("cosign-certificate-oidc-issuer-regexp"); err != nil {
		return
	}
	return
}

func ValidateHealthcheckFlags(options types.ContainerCreateOptions) error {
	healthFlagsSet :=
		options.HealthInterval != 0 ||
			options.HealthTimeout != 0 ||
			options.HealthRetries != 0 ||
			options.HealthStartPeriod != 0 ||
			options.HealthStartInterval != 0

	if options.NoHealthcheck {
		if options.HealthCmd != "" || healthFlagsSet {
			return fmt.Errorf("--no-healthcheck conflicts with --health-* options")
		}
	}

	// Note: HealthCmd can be empty with other healthcheck flags set cause healthCmd could be coming from image.
	if options.HealthInterval < 0 {
		return fmt.Errorf("--health-interval cannot be negative")
	}
	if options.HealthTimeout < 0 {
		return fmt.Errorf("--health-timeout cannot be negative")
	}
	if options.HealthRetries < 0 {
		return fmt.Errorf("--health-retries cannot be negative")
	}
	if options.HealthStartPeriod < 0 {
		return fmt.Errorf("--health-start-period cannot be negative")
	}
	if options.HealthStartInterval < 0 {
		return fmt.Errorf("--health-start-interval cannot be negative")
	}
	return nil
}

func ProcessRootCmdFlags(cmd *cobra.Command) (types.GlobalCommandOptions, error) {
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	debugFull, err := cmd.Flags().GetBool("debug-full")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	address, err := cmd.Flags().GetString("address")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	namespace, err := cmd.Flags().GetString("namespace")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	snapshotter, err := cmd.Flags().GetString("snapshotter")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	cniPath, err := cmd.Flags().GetString("cni-path")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	cniConfigPath, err := cmd.Flags().GetString("cni-netconfpath")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	dataRoot, err := cmd.Flags().GetString("data-root")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	cgroupManager, err := cmd.Flags().GetString("cgroup-manager")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	insecureRegistry, err := cmd.Flags().GetBool("insecure-registry")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	hostsDir, err := cmd.Flags().GetStringSlice("hosts-dir")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	experimental, err := cmd.Flags().GetBool("experimental")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	hostGatewayIP, err := cmd.Flags().GetString("host-gateway-ip")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	bridgeIP, err := cmd.Flags().GetString("bridge-ip")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	kubeHideDupe, err := cmd.Flags().GetBool("kube-hide-dupe")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	cdiSpecDirs, err := cmd.Flags().GetStringSlice("cdi-spec-dirs")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	dns, err := cmd.Flags().GetStringSlice("global-dns")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	dnsOpts, err := cmd.Flags().GetStringSlice("global-dns-opts")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}
	dnsSearch, err := cmd.Flags().GetStringSlice("global-dns-search")
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}

	// Point to dataRoot for filesystem-helpers implementing rollback / backups.
	err = pkg.InitFS(dataRoot)
	if err != nil {
		return types.GlobalCommandOptions{}, err
	}

	return types.GlobalCommandOptions{
		Debug:            debug,
		DebugFull:        debugFull,
		Address:          address,
		Namespace:        namespace,
		Snapshotter:      snapshotter,
		CNIPath:          cniPath,
		CNINetConfPath:   cniConfigPath,
		DataRoot:         dataRoot,
		CgroupManager:    cgroupManager,
		InsecureRegistry: insecureRegistry,
		HostsDir:         hostsDir,
		Experimental:     experimental,
		HostGatewayIP:    hostGatewayIP,
		BridgeIP:         bridgeIP,
		KubeHideDupe:     kubeHideDupe,
		CDISpecDirs:      cdiSpecDirs,
		DNS:              dns,
		DNSOpts:          dnsOpts,
		DNSSearch:        dnsSearch,
	}, nil
}

func CheckExperimental(feature string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		globalOptions, err := ProcessRootCmdFlags(cmd)
		if err != nil {
			return err
		}
		if !globalOptions.Experimental {
			return fmt.Errorf("%s is experimental feature, you should enable experimental config", feature)
		}
		return nil
	}
}
