/*
Copyright 2018 The Kubernetes Authors.

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

package provision

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cloud-provider-vsphere/pkg/cli"
)

var (
	configFile      string
	interactive     bool
	oldInTreeConfig bool

	// vCenter IP.
	vchost string
	// vCenter port.
	vcport string
	// True if vCenter uses self-signed cert.
	insecure bool
	// Datacenter in which VMs are located.
	datacenter string
	// Name of the secret were vCenter credentials are present.
	secretName string
	// Secret Namespace where secret will be present that has vCenter credentials.
	secretNamespace string
	// vCenter username.
	vcUser string
	// vCenter password in clear text.
	vcPassword string
	// vCenter certificate
	vcCert string
	// vcRole is role for solution user (Default is Administrator)
	vcRole string
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Initialize provisioning with vSphere cloud provider",
	Long: `Starting prerequisites for deploying a cloud provider on vSphere, in cluding :
	[x] vSphere configuration health check.
	[x] Create vSphere solution user.
	[x] Create vSohere role with minimal set of premissions.
  `,
	Example: `# Specify interaction mode or declaration mode (default)
	vcpctl provision --interactive=false
`,
	Run: RunProvision,
}

func AddProvision(cmd *cobra.Command) {

	provisionCmd.Flags().StringVar(&configFile, "config", "", "VSphere cloud provider config file path")
	provisionCmd.Flags().BoolVar(&oldInTreeConfig, "oldConfig", false, "Old int-tree vsphere configuration file, true or false")
	provisionCmd.Flags().BoolVar(&interactive, "interactive", true, "Specify interactive mode (true) as default, set (false) for declarative mode for automation")

	provisionCmd.Flags().StringVar(&vchost, "host", "", "Specify vCenter IP")
	provisionCmd.Flags().StringVar(&vcport, "port", "", "Specify vCenter Port")
	provisionCmd.Flags().StringVar(&vcUser, "user", "", "Specify vCenter user")
	provisionCmd.Flags().StringVar(&vcPassword, "password", "", "Specify vCenter Password")
	provisionCmd.Flags().BoolVar(&insecure, "insecure", false, "Don't verify the server's certificate chain")
	provisionCmd.Flags().StringVar(&vcCert, "cert", "", "Certificate for solution user")
	provisionCmd.Flags().StringVar(&vcRole, "role", "Administrator", "Role for solution user (RegularUser|Administrator)")

	cmd.AddCommand(provisionCmd)
}

func RunProvision(cmd *cobra.Command, args []string) {
	// TODO (fanz): implement provision
	fmt.Println("Perform cloud provider provisioning...")
	o := cli.ClientOption{}
	o.LoadCredential(vcUser, vcPassword, vcCert, vcRole)
	ctx := context.Background()
	client, err := o.NewClient(ctx, vchost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	o.Client = client
	cli.CreateSolutionUser(ctx, &o)
}
