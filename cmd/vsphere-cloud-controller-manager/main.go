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

// The external controller manager is responsible for running controller loops that
// are cloud provider dependent. It uses the API to listen to new events on resources.

package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	_ "k8s.io/kubernetes/pkg/util/prometheusclientgo" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"      // for version metric registration

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
)

// AppName is the full name of this CCM
const AppName string = "vsphere-cloud-controller-manager"

var version string

func main() {
	loadbalancer.Version = version
	loadbalancer.AppName = AppName

	rand.Seed(time.Now().UTC().UnixNano())

	command := app.NewCloudControllerManagerCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(flag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Infof("vsphere-cloud-controller-manager version: %s", version)

	// Set cloud-provider flag to vsphere
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "cloud-provider" {
			flag.Value.Set(vsphere.ProviderName)
			flag.DefValue = vsphere.ProviderName
			return
		}
	})

	var versionFlag *pflag.Value
	var clusterNameFlag *pflag.Value
	pflag.CommandLine.VisitAll(func(flag *pflag.Flag) {
		switch flag.Name {
		case "cluster-name":
			clusterNameFlag = &flag.Value
		case "version":
			versionFlag = &flag.Value
		}
	})

	command.Use = AppName
	innerRun := command.Run
	command.Run = func(cmd *cobra.Command, args []string) {
		if versionFlag != nil && (*versionFlag).String() != "false" {
			fmt.Printf("%s %s\n", AppName, version)
			os.Exit(0)
		}
		if clusterNameFlag != nil {
			loadbalancer.ClusterName = (*clusterNameFlag).String()
		}
		innerRun(cmd, args)
	}

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
