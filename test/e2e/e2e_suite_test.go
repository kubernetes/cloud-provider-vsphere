/*
Copyright 2021 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	helmrelease "helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

// Test Suite flags
var (
	// configPath is the path to the e2e config file.
	configPath string

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// chartFolder is the folder to store vsphere-cpi chart for testing
	chartFolder string

	// clusterctlConfig is the file which tests will use as a clusterctl config.
	// If it is not set, a local clusterctl repository (including a clusterctl config) will be created automatically.
	clusterctlConfig string

	// version is the cloud-controller-manager version to be tested, for example, v1.22.3-76-g6f4fa01
	version string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

var (
	// kubeconfig for the workload cluster
	workloadKubeconfigNamespace                    = "default"
	workloadKubeconfig                             = "/tmp/wl.kubeconfig"
	workloadKubeconfigSecret                       = corev1.Secret{}
	workloadRestConfig          *restclient.Config = nil
	workloadClientset           *kubernetes.Clientset

	// helm install configurations
	namespace = "kube-system"
	release   = "vsphere-cpi-e2e"
	image     = "gcr.io/cloud-provider-vsphere/cpi/pr/manager"

	// helm install expectation
	daemonsetName = "vsphere-cpi"
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.StringVar(&clusterctlConfig, "e2e.clusterctl-config", "", "file which tests will use as a clusterctl config. If it is not set, a local clusterctl repository (including a clusterctl config) will be created automatically.")
	flag.StringVar(&chartFolder, "e2e.chart-folder", "", "folder where the helm chart for e2e should be stored")
	flag.StringVar(&version, "e2e.version", "dev", "the cloud-controller-manager version to be tested, for example, v1.22.3-76-g6f4fa01")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false,
		"if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")

}

// Global variables
var (
	ctx = context.Background()
	err error

	e2eConfig            *clusterctl.E2EConfig
	vsphere              *VSphereTestClient
	clusterctlConfigPath string // path to the clusterctl config file

	provider   bootstrap.ClusterProvider
	proxy      framework.ClusterProxy
	kubeconfig string

	workloadName   string
	workloadResult *clusterctl.ApplyClusterTemplateAndWaitResult
)

func defaultScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	return sc
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "vsphere-cpi-e2e")
}

// Create a kind cluster that shared across all the tests
var _ = SynchronizedBeforeSuite(func() []byte {
	By("Load e2e config file", func() {
		Expect(configPath).To(BeAnExistingFile(), "invalid test suite argument. e2e.config should be an existing file.")
		e2eConfig = clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
		Expect(e2eConfig).NotTo(BeNil(), "cannot load e2e config file from ", configPath)
	})

	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gosec

	By("Ensure clusterctl config", func() {
		if clusterctlConfig == "" {
			clusterctlConfigPath = createClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"))
		} else {
			clusterctlConfigPath = clusterctlConfig
		}
	})

	By("Init vSphere session", func() {
		vsphere, err = initVSphereTestClient(ctx, e2eConfig)
		Expect(err).Should(BeNil())
		Expect(vsphere).NotTo(BeNil())
	})

	By("Setup bootstrap cluster", func() {
		provider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               e2eConfig.ManagementClusterName,
			RequiresDockerSock: e2eConfig.HasDockerProvider(),
			Images:             e2eConfig.Images,
		})
		Expect(provider).NotTo(BeNil())

		kubeconfig = provider.GetKubeconfigPath()
		Expect(kubeconfig).NotTo(BeEmpty())
		Expect(kubeconfig).To(BeAnExistingFile(), "kubeconfig for the boostrap cluster does not exist")

		proxy = framework.NewClusterProxy("bootstrap", kubeconfig, defaultScheme())
		Expect(proxy).NotTo(BeNil())
	})

	By("Initialize bootstrap cluster", func() {
		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:            proxy,
			ClusterctlConfigPath:    clusterctlConfigPath,
			LogFolder:               filepath.Join(artifactFolder, "clusters", proxy.GetName()),
			InfrastructureProviders: e2eConfig.InfrastructureProviders(),
		}, e2eConfig.GetIntervals(proxy.GetName(), "wait-controllers")...)
	})

	By("Create a workload cluster", func() {
		workloadName = fmt.Sprintf("%s-%s", "workload", util.RandomString(6))
		workloadResult = new(clusterctl.ApplyClusterTemplateAndWaitResult)
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: proxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", proxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           proxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				ClusterName:              workloadName,
				Namespace:                workloadKubeconfigNamespace,
				KubernetesVersion:        e2eConfig.GetVariable("INIT_WITH_KUBERNETES_VERSION"),
				ControlPlaneMachineCount: e2eConfig.GetInt64PtrVariable("CONTROL_PLANE_MACHINE_COUNT"),
				WorkerMachineCount:       e2eConfig.GetInt64PtrVariable("WORKER_MACHINE_COUNT"),
				Flavor:                   clusterctl.DefaultFlavor,
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(proxy.GetName(), "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(proxy.GetName(), "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(proxy.GetName(), "wait-worker-nodes"),
		}, workloadResult)
		klog.Infof("Created workload cluster %s\n", workloadName)
	})

	By("Grab workload cluster kubeconfig", func() {
		err := proxy.GetClient().Get(ctx, types.NamespacedName{
			Namespace: workloadKubeconfigNamespace,
			Name:      workloadName + "-kubeconfig",
		}, &workloadKubeconfigSecret)
		if err != nil {
			Fail("Cannot retrieve workload cluster kubeconfig")
		}
		err = writeSecretKubeconfigToFile(&workloadKubeconfigSecret, "value", workloadKubeconfig)
		Expect(err).NotTo(HaveOccurred())
		workloadRestConfig, err = clientcmd.BuildConfigFromFlags("", workloadKubeconfig)
		Expect(err).NotTo(HaveOccurred())
		workloadClientset, err = kubernetes.NewForConfig(workloadRestConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	By("Wait for workload cluster to come up", func() {
		Eventually(func() error {
			_, err := workloadClientset.ServerVersion()
			return err
		}).Should(BeNil())
	})

	By("Remove old vsphere-cpi", func() {
		Eventually(func() error {
			return removeOldCPI(workloadClientset)
		}, 2*time.Minute).Should(BeNil())
	})

	By("Install new vsphere-cpi with helm on workload cluster", func() {
		actionConfig := new(action.Configuration)
		err = actionConfig.Init(kube.GetConfig(workloadKubeconfig, "", namespace), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {})
		Expect(err).NotTo(HaveOccurred())

		chart, err := loader.Load(chartFolder)
		Expect(err).NotTo(HaveOccurred())

		install := newCPIInstallFromConfig(actionConfig)
		values := newCPIInstallValues()

		var release *helmrelease.Release
		Eventually(func() error {
			release, err = install.Run(chart, values)
			return err
		}).ShouldNot(HaveOccurred(), "Cannot install vsphere-cpi helm chart")
		klog.Infof("Installed %s helm chart in namespace %s\n", release.Name, release.Namespace)
	})

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			proxy.GetKubeconfigPath(),
		}, ","))
}, func(data []byte) {
	// before each parallel thread
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	// after all parallel test cases finish
	if !skipCleanup {
		By("Tear down the workload cluster", func() {
			framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
				Client:    proxy.GetClient(),
				Namespace: "default",
			}, e2eConfig.GetIntervals(proxy.GetName(), "wait-delete-cluster")...)
			klog.Infof("Deleted workload cluster %s/%s\n", workloadResult.Cluster.Namespace, workloadResult.Cluster.Name)
		})
		By("Tear down the bootstrap cluster", func() {
			Expect(provider).NotTo(BeNil())
			Expect(proxy).NotTo(BeNil())

			provider.Dispose(ctx)
			proxy.Dispose(ctx)
		})
	}
})

// createClusterctlLocalRepository ensures a repository for `config` is created at path repositoryFolder
func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file in place of the CNI_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(e2e.CNIPath), "Missing %s variable in the config", e2e.CNIPath)
	cniPath := config.GetVariable(e2e.CNIPath)
	Expect(cniPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", e2e.CNIPath)

	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, e2e.CNIResources)

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)
	return clusterctlConfig
}

// writeSecretKubeconfigToFile dumps the kubeconfig from secret to a file
func writeSecretKubeconfigToFile(secret *corev1.Secret, key string, file string) error {
	if secret == nil {
		return errors.New("secret is nil")
	}
	val, exists := secret.Data[key]
	klog.Infof("workload kubeconfig:\n%s\n", val)
	if !exists {
		return errors.New("key does not exist in the secret")
	}
	return os.WriteFile(file, val, 0644)
}

// removeOldCPI removes the old vsphere-cpi instance before installing a build version using helm
func removeOldCPI(clientset *kubernetes.Clientset) error {
	if err := clientset.AppsV1().DaemonSets(namespace).Delete(ctx, "vsphere-cloud-controller-manager", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, "cloud-controller-manager", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.RbacV1().RoleBindings(namespace).Delete(ctx, "servicecatalog.k8s.io:apiserver-authentication-reader", metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// newCPIInstallFromConfig returns an `Install` object, given the configurations, for the CPI chart installation
func newCPIInstallFromConfig(config *action.Configuration) *action.Install {
	install := action.NewInstall(config)
	install.ReleaseName = release
	install.Namespace = namespace
	install.DryRun = false
	return install
}

// newCPIInstallValues returns the values to helm-install the CPI chart
func newCPIInstallValues() map[string]interface{} {
	values := map[string]interface{}{
		"config": map[string]interface{}{
			"enabled":    "true",
			"name":       "cloud-config",
			"vcenter":    e2eConfig.GetVariable("VSPHERE_SERVER"),
			"username":   e2eConfig.GetVariable("VSPHERE_USERNAME"),
			"password":   e2eConfig.GetVariable("VSPHERE_PASSWORD"),
			"datacenter": e2eConfig.GetVariable("VSPHERE_DATACENTER"),
			"region":     "",
			"zone":       "",
		},
		"daemonset": map[string]interface{}{
			"image": image,
			"tag":   version,
		},
		"securityContext": map[string]interface{}{
			"enabled": "false",
		},
	}
	return values
}
