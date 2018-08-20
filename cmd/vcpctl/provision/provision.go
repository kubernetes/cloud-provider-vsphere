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
	"fmt"

	"github.com/spf13/cobra"
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
	provisionCmd.Flags().StringVar(&configFile, "config", "", "vsphere cloud provider config file path")
	provisionCmd.Flags().BoolVar(&oldInTreeConfig, "oldConfig", false, "old int-tree vsphere configuration file, true or false")
	provisionCmd.Flags().BoolVar(&interactive, "interactive", true, "specify interactive mode (true) as default, set (false) for declarative mode for automation")

	provisionCmd.Flags().StringVar(&vchost, "vchost", "", "specify vCenter IP ")
	provisionCmd.Flags().StringVar(&vcport, "vcport", "", "specify vCenter Port ")
	provisionCmd.Flags().StringVar(&vcUser, "vcuser", "", "specify vCenter user ")
	provisionCmd.Flags().StringVar(&vcPassword, "vcpassword", "", "specify vCenter Password ")
	provisionCmd.Flags().BoolVar(&insecure, "insecure", false, "Don't verify the server's certificate chain")
	provisionCmd.Flags()

	cmd.AddCommand(provisionCmd)
}

func RunProvision(cmd *cobra.Command, args []string) {
	// TODO (fanz): implement provision
	fmt.Println("Perform cloud provider provisioning...")

}
