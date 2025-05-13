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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
	"sigs.k8s.io/cluster-api/util"

	ctrl "sigs.k8s.io/controller-runtime"

	vsphereframework "sigs.k8s.io/cluster-api-provider-vsphere/test/framework"
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

	// image is the cloud-controller-manager image to be tested, for example, gcr.io/k8s-staging-cloud-pv-vsphere/cloud-provider-vsphere
	image string

	// version is the cloud-controller-manager version to be tested, for example, v1.22.3-76-g6f4fa01
	version string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// useLatestK8sVersion indicates if the e2e test should use k8s version specified in KUBERNETES_VERSION_LATEST_CI
	useLatestK8sVersion bool
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

	// helm install expectation
	daemonsetName = "vsphere-cpi"
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.StringVar(&clusterctlConfig, "e2e.clusterctl-config", "", "file which tests will use as a clusterctl config. If it is not set, a local clusterctl repository (including a clusterctl config) will be created automatically.")
	flag.StringVar(&chartFolder, "e2e.chart-folder", "", "folder where the helm chart for e2e should be stored")
	flag.StringVar(&image, "e2e.image", "gcr.io/k8s-staging-cloud-pv-vsphere/cloud-provider-vsphere", "the cloud-controller-manager image to be tested, for example, gcr.io/k8s-staging-cloud-pv-vsphere/cloud-provider-vsphere")
	flag.StringVar(&version, "e2e.version", "dev", "the cloud-controller-manager version to be tested, for example, v1.22.3-76-g6f4fa01")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false,
		"if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useLatestK8sVersion, "e2e.use-latest-k8s-version", false, "if true, e2e test suite will run on a k8s version specified in KUBERNETES_VERSION_LATEST_CI")
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
	// This line prevents controller-runtime from complaining about log.SetLogger never being called
	ctrl.SetLogger(klog.Background())

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

	workloadName = fmt.Sprintf("%s-%s", "workload", util.RandomString(6))
	By("Create a workload cluster", func() {
		workloadInput := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: proxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", proxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           proxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				ClusterName:              workloadName,
				Namespace:                workloadKubeconfigNamespace,
				ControlPlaneMachineCount: e2eConfig.MustGetInt64PtrVariable("CONTROL_PLANE_MACHINE_COUNT"),
				WorkerMachineCount:       e2eConfig.MustGetInt64PtrVariable("WORKER_MACHINE_COUNT"),
				Flavor:                   clusterctl.DefaultFlavor,
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(proxy.GetName(), "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(proxy.GetName(), "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(proxy.GetName(), "wait-worker-nodes"),
		}
		workloadResult = &clusterctl.ApplyClusterTemplateAndWaitResult{}
		resolveK8sVersion()
		// if use dev k8s version, fast-rollout is needed to install Kubernetes on bootstrap
		if useLatestK8sVersion {
			workloadInput.ConfigCluster.Flavor = "fast-rollout"
		} else {
			workloadInput.ConfigCluster.KubernetesVersion = e2eConfig.MustGetVariable("KUBERNETES_VERSION")
		}
		clusterctl.ApplyClusterTemplateAndWait(ctx, workloadInput, workloadResult)
		klog.Infof("Created k8s %s workload cluster %s\n", e2eConfig.MustGetVariable("KUBERNETES_VERSION"), workloadName)
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

	By("Load vsphere-cpi image", func() {
		vsphereframework.LoadImagesFunc(ctx)(proxy.GetWorkloadCluster(ctx, workloadKubeconfigNamespace, workloadName))
	})

	By("Install dev vsphere cpi using helm on workload cluster", func() {
		cmdName := "helm"
		cmdArgs := []string{
			"install", "vsphere-cpi", "vsphere-cpi/vsphere-cpi",
			"--namespace", namespace,
			"--set", "config.enabled=true",
			"--set", "config.name=cloud-config",
			"--set", "config.vcenter=" + e2eConfig.MustGetVariable("VSPHERE_SERVER"),
			"--set", "config.username=" + e2eConfig.MustGetVariable("VSPHERE_USERNAME"),
			"--set", "config.password=" + e2eConfig.MustGetVariable("VSPHERE_PASSWORD"),
			"--set", "config.datacenter=" + e2eConfig.MustGetVariable("VSPHERE_DATACENTER"),
			"--set", "config.region=" + "",
			"--set", "config.zone=" + "",
			"--set", "daemonset.image=" + image,
			"--set", "daemonset.tag=" + version,
			"--set", "securityContext.enabled=false",
		}

		// Create the command
		cmd := exec.Command(cmdName, cmdArgs...)
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", workloadKubeconfig))

		// Capture the output (stdout and stderr)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		klog.Infof("Command output: %s\n", string(output))
	})

	By("Watching vsphere-cpi daemonset logs", func() {
		workloadProxy := proxy.GetWorkloadCluster(ctx, workloadKubeconfigNamespace, workloadName)

		framework.WatchDaemonSetLogsByLabelSelector(ctx, framework.WatchDaemonSetLogsByLabelSelectorInput{
			GetLister: workloadProxy.GetClient(),
			Cache:     workloadProxy.GetCache(ctx),
			ClientSet: workloadProxy.GetClientSet(),
			Labels: map[string]string{
				"component": "cloud-controller-manager",
			},
			LogPath: filepath.Join(artifactFolder, "clusters", workloadProxy.GetName(), "logs"),
		})
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
		By("Dump all resources to artifacts", func() {
			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:               proxy.GetClient(),
				KubeConfigPath:       proxy.GetKubeconfigPath(),
				ClusterctlConfigPath: clusterctlConfigPath,
				Namespace:            "default",
				LogPath:              filepath.Join(artifactFolder, "clusters", proxy.GetName(), "resources"),
			})
		})
		By("Tear down the workload cluster", func() {
			Expect(workloadResult).NotTo(BeNil())
			framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
				ClusterProxy:         proxy,
				ClusterctlConfigPath: clusterctlConfigPath,
				Namespace:            "default",
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
	cniPath := config.MustGetVariable(e2e.CNIPath)
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
	if err := clientset.AppsV1().DaemonSets(namespace).Delete(ctx, "vsphere-cpi", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, "cloud-config", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, "cloud-controller-manager", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.RbacV1().ClusterRoles().Delete(ctx, "cloud-controller-manager", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(ctx, "cloud-controller-manager", metav1.DeleteOptions{}); err != nil {
		return err
	}
	if err := clientset.RbacV1().RoleBindings(namespace).Delete(ctx, "servicecatalog.k8s.io:apiserver-authentication-reader", metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// resolveK8sVersion valids and sets the correct k8s from KUBERNETES_VERSION_LATEST_CI or KUBERNETES_VERSION
func resolveK8sVersion() {
	var kubernetesVersion string
	var err error
	if useLatestK8sVersion {
		kubernetesVersion, err = kubernetesversions.ResolveVersion(ctx, e2eConfig.MustGetVariable("KUBERNETES_VERSION_LATEST_CI"))
	} else {
		kubernetesVersion, err = kubernetesversions.ResolveVersion(ctx, e2eConfig.MustGetVariable("KUBERNETES_VERSION"))
	}
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Setenv("KUBERNETES_VERSION", kubernetesVersion)).To(Succeed())
}
