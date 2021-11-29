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
	"strings"
	"time"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual"
	"k8s.io/cloud-provider/app"
	appconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // for client metrics registration
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"
	klog "k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AppName is the full name of this CCM
const AppName string = "vsphere-cloud-controller-manager"

var version string

func main() {
	loadbalancer.Version = version
	loadbalancer.AppName = AppName

	rand.Seed(time.Now().UTC().UnixNano())

	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	var controllerInitializers map[string]app.InitFunc
	command := &cobra.Command{
		Use:  "vsphere-cloud-controller-manager",
		Long: `vsphere-cloud-controller-manager manages vSphere cloud resources for a Kubernetes cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			cliflag.PrintFlags(cmd.Flags())

			c, err := ccmOptions.Config(app.ControllerNames(app.DefaultInitFuncConstructors), app.ControllersDisabledByDefault.List())
			if err != nil {
				// explicitly ignore the error by Fprintf, exiting anyway
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			klog.Infof("vsphere-cloud-controller-manager version: %s", version)

			// Default to the vsphere cloud provider if not set
			cloudProviderFlag := cmd.Flags().Lookup("cloud-provider")
			if cloudProviderFlag.Value.String() == "" {
				if err := cloudProviderFlag.Value.Set(vsphere.RegisteredProviderName); err != nil {
					klog.Fatalf("cannot set RegisteredProviderName to %s: %v", vsphere.RegisteredProviderName, err)
				}
			}

			cloudProvider := cloudProviderFlag.Value.String()
			if cloudProvider != vsphere.RegisteredProviderName && cloudProvider != vsphereparavirtual.RegisteredProviderName {
				klog.Fatalf("unknown cloud provider %s, only 'vsphere' and 'vsphere-paravirtual' are supported", cloudProvider)
			}

			completedConfig := c.Complete()

			cloud := initializeCloud(completedConfig, cloudProvider)
			controllerInitializers = app.ConstructControllerInitializers(app.DefaultInitFuncConstructors, completedConfig, cloud)

			// initialize a notifier for cloud config update
			cloudConfig := completedConfig.ComponentConfig.KubeCloudShared.CloudProvider.CloudConfigFile
			klog.Infof("initialize notifier on configmap update %s\n", cloudConfig)
			watch, stop, err := initializeWatch(completedConfig, cloudConfig)
			if err != nil {
				klog.Fatalf("fail to initialize watch on config map %s: %v\n", cloudConfig, err)
			}
			defer func(watch *fsnotify.Watcher) {
				_ = watch.Close() // ignore explicitly when the watch closes
			}(watch)

			if err := app.Run(completedConfig, cloud, controllerInitializers, stop); err != nil {
				// explicitly ignore the error by Fprintf, exiting anyway due to app error
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	fs := command.Flags()
	namedFlagSets := ccmOptions.Flags(app.ControllerNames(app.DefaultInitFuncConstructors), app.ControllersDisabledByDefault.List())
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), command.Name())

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(command.OutOrStdout())
	command.SetUsageFunc(func(cmd *cobra.Command) error {
		if _, err := fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine()); err != nil {
			return err
		}
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	command.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine()); err != nil {
			return
		}
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	var clusterNameFlag *pflag.Value
	var controllersFlag *pflag.Value
	var cloudProviderFlag *pflag.Value
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		switch flag.Name {
		// Set cloud-provider flag to vsphere
		case "cloud-provider":
			cloudProviderFlag = &flag.Value
			if err := flag.Value.Set(vsphere.RegisteredProviderName); err != nil {
				klog.Fatalf("cannot set RegisteredProviderName to %s: %v", vsphere.RegisteredProviderName, err)
			}
			flag.DefValue = vsphere.RegisteredProviderName
		case "cluster-name":
			clusterNameFlag = &flag.Value
		case "controllers":
			controllersFlag = &flag.Value
		}
	})

	var versionFlag *pflag.Value
	pflag.CommandLine.VisitAll(func(flag *pflag.Flag) {
		switch flag.Name {
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
			vsphereparavirtual.ClusterName = (*clusterNameFlag).String()
		}
		// if route controller is enabled in vsphereparavirtual cloud provider, set routeEnabled to true
		if controllersFlag != nil &&
			!strings.Contains((*controllersFlag).String(), "-route") &&
			(strings.Contains((*controllersFlag).String(), "route") || strings.Contains((*controllersFlag).String(), "*")) &&
			vsphereparavirtual.RegisteredProviderName == (*cloudProviderFlag).String() {
			vsphereparavirtual.RouteEnabled = true
		}
		innerRun(cmd, args)
	}

	if err := command.Execute(); err != nil {
		// ignore error by Fprintf, exit anyway due to cmd execute error
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// set up a filesystem watcher for the cloud config mount
// reboot the app whenever there is an update via the returned stopCh
func initializeWatch(_ *appconfig.CompletedConfig, cloudConfigPath string) (watch *fsnotify.Watcher, stopCh chan struct{}, err error) {
	stopCh = make(chan struct{})
	watch, err = fsnotify.NewWatcher()
	if err != nil {
		klog.Fatalln("fail to setup config watcher")
	}
	go func() {
		for {
			select {
			case err := <-watch.Errors:
				klog.Warningf("watcher receives err: %v\n", err)
			case event := <-watch.Events:
				klog.Fatalf("config map %s has been updated, restarting pod, received event %v\n", cloudConfigPath, event)
				stopCh <- struct{}{}
			}
		}
	}()
	if err := watch.Add(cloudConfigPath); err != nil {
		klog.Fatalf("fail to watch cloud config file %s\n", cloudConfigPath)
	}
	return
}

func initializeCloud(config *appconfig.CompletedConfig, cloudProvider string) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudProvider, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}
	return cloud
}
