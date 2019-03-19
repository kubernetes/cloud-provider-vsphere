# Continuous integration

The image `gcr.io/cloud-provider-vsphere/ci` is used by Prow jobs to build, test, and deploy the CCM and CSI providers.

## The CI workflow

Prow jobs are configured to perform the following steps:

| Job type | Linters | Build binaries | Unit test | Build images | Integration test | Deploy images | Conformance test |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Presubmit | ✓ | ✓ | ✓ | ✓ | ✓ | | |
| Postsubmit | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Periodic | ✓ | ✓ | ✓ | ✓ | | | ✓ |

## Up-to-date sources

When running on Prow the jobs map the current sources into the CI container. That may be simulated locally by running the examples from a directory containing the desired sources and providing the `docker run` command with the following flags:

* `-v "$(pwd)":/go/src/k8s.io/cloud-provider-vsphere`

## Docker-in-Docker

Several of the jobs require Docker-in-Docker. To mimic that locally there are two options:

1. [Provide the host's Docker to the container](#provide-the-hosts-docker-to-the-container) 
2. [Run the Docker server inside the container](#run-the-docker-server-inside-the-container)

### Provide the host's Docker to the container

While Prow jobs [run the Docker server inside the container](#run-the-docker-server-inside-the-container), this option provides a low-cost (memory, disk) solution for testing locally. This option is enabled by running the examples from a directory containing the desired sources and providing the `docker run` command with the following flags:

* `-v /var/run/docker.sock:/var/run/docker.sock`
* `-e "PROJECT_ROOT=$(pwd)"`
* `-v "$(pwd)":/go/src/k8s.io/cloud-provider-vsphere`

Please note that this option is only available when using a local copy of the sources. This is because all of the paths known to Docker will be of the local host system, not from the container. That's also why it's necessary to provide the `PROJECT_ROOT` environment variable -- it indicates to certain recipes the location of specific files or directories relative to the local sources on the host system.

### Run the Docker server inside the container
This is option that Prow jobs utilize and is also the method illustrated by the examples below. Please keep in mind that using this option locally requires a large amount of memory and disk space available to Docker:

| Type | Minimum Requirement |
|------|---------------------|
| Memory | 8GiB |
| Disk | 200GiB |

For Windows and macOS systems this means adjusting the size of the Docker VM disk and the amount of memory the Docker VM is allowed to use.

Resources notwithstanding, running the Docker server inside the container also requires providing the `docker run` command with the following flags:

* `--privileged`

## Check the sources

To check the sources run the following command:

```shell
$ docker run -it --rm \
  -e "ARTIFACTS=/out" -v "$(pwd)":/out \
  gcr.io/cloud-provider-vsphere/ci \
  make check
```

The above command will create the following files in the working directory:

* `junit_check.xml`

## Build the CCM and CSI binaries

The CI image is built with Go module and build caches from a recent build of the project's `master` branch. Therefore the CI image can be used to build the CCM and CSI binaries in a matter of seconds:

```shell
$ docker run -it --rm \
  -e "BIN_OUT=/out" -v "$(pwd)":/out \
  gcr.io/cloud-provider-vsphere/ci \
  make build
```

The above command will create the following files in the working directory:

* `vsphere-cloud-controller-manager.linux_amd64`
* `vsphere-csi.linux_amd64`

## Execute the unit tests

```shell
$ docker run -it --rm \
  gcr.io/cloud-provider-vsphere/ci \
  make unit-test
```

## Build the CCM and CSI images

Building the CCM and CSI images inside another image requires Docker-in-Docker (DinD):

```shell
$ docker run -it --rm --privileged \
  gcr.io/cloud-provider-vsphere/ci \
  make build-images
```

## Execute the integration tests
The project's integration tests leverage Kind, a solution for turning up a Kubernetes cluster using Docker:

```shell
$ docker run -it --rm --privileged \
  gcr.io/cloud-provider-vsphere/ci \
  make integration-test
```

Running the integration tests with the container providing the Docker server is **severely** taxing on the host system's resources. It is **highly** recommended, for purposes of local development, to opt to provide Docker to the container by bind mounting the host's Docker socket into the container. Please note this also requires using local sources and setting `PROJECT_ROOT`:

```shell
$ docker run -it --rm \
  -e "PROJECT_ROOT=$(pwd)" \
  -v "$(pwd)":/go/src/k8s.io/cloud-provider-vsphere \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gcr.io/cloud-provider-vsphere/ci \
  make integration-test
```

## Deploy the CCM and CSI images
Pushing the images requires bind mounting a GCR key file into the container and setting the environment variable `GCR_KEY_FILE` to inform the deployment process the location of the key file:

```shell
$ docker run -it --rm --privileged \
  -e "GCR_KEY_FILE=/keyfile.json" -v "$(pwd)/keyfile.json":/keyfile.json \
  gcr.io/cloud-provider-vsphere/ci \
  make push-images
```

## Execute the conformance tests
Running the e2e conformance suite not only requires DinD but also an environment variable file that provides the information required to turn up a Kubernetes cluster against which the e2e tests are executed. For example:

```shell
VSPHERE_SERVER='vcenter.com'
VSPHERE_USERNAME='myuser'
VSPHERE_PASSWORD='mypass'
VSPHERE_DATACENTER='/dc1'
VSPHERE_RESOURCE_POOL="/dc1/host/Cluster-1/Resources/mypool"
VSPHERE_DATASTORE="/dc1/datastore/mydatastore"
VSPHERE_FOLDER="/dc1/vm/myfolder"
```

If the vSphere endpoint is hosted in the VMware Cloud (VMC) on AWS then the file can also contain AWS access credentials to provide external access to the Kubernetes cluster:

```shell
AWS_ACCESS_KEY_ID='mykey'
AWS_SECRET_ACCESS_KEY='mysecretkey'
AWS_DEFAULT_REGION='myregion'
```

Finally, the configuration file can also include details that define the shape of the Kubernetes cluster as well as influence how and what e2e tests are executed:

```shell
CLOUD_PROVIDER='external'
E2E_FOCUS='\\[Conformance\\]'
E2E_SKIP='Alpha|\\[(Disruptive|Feature:[^\\]]+|Flaky)\\]'
K8S_VERSION='ci/latest'
NUM_BOTH='1'
NUM_CONTROLLERS='1'
NUM_WORKERS='1'
KUBE_CONFORMANCE_IMAGE='akutz/kube-conformance:latest'
```

Once the environment variable file is created, the conformance tests may be executed with:

```shell
$ docker run -it --rm --privileged \
  -e "ARTIFACTS=/out" -v "$(pwd)":/out \
  --env-file config.env \
  gcr.io/cloud-provider-vsphere/ci \
  make conformance-test
```

The above command will create the following files in the working directory:

```shell
$ ls -al
total 1936
drwxr-xr-x  13 akutz  staff   416B Mar 15 17:26 ./
drwxr-xr-x   3 akutz  staff    96B Mar 15 17:21 ../
-rw-r--r--   1 akutz  staff   599B Mar 15 17:21 build-info.json
-rw-r--r--   1 akutz  staff   8.4K Mar 15 17:26 e2e.log
drwxr-xr-x@  4 akutz  staff   128B Mar 15 17:26 hosts/
-rw-r--r--   1 akutz  staff   935K Mar 15 17:26 junit_01.xml
drwxr-xr-x@  5 akutz  staff   160B Mar 15 17:26 meta/
drwxr-xr-x@  4 akutz  staff   128B Mar 15 17:26 plugins/
drwxr-xr-x@  3 akutz  staff    96B Mar 15 17:26 podlogs/
drwxr-xr-x@  4 akutz  staff   128B Mar 15 17:26 resources/
-rw-r--r--@  1 akutz  staff   4.0K Mar 15 17:26 servergroups.json
-rw-r--r--@  1 akutz  staff   253B Mar 15 17:26 serverversion.json
-rw-r--r--   1 akutz  staff   281B Mar 15 17:25 terraform-output-vars.txt
```

The `build-info.json` file includes information about the build:

```json
{
  "cluster-name": "prow-fb3ba63",
  "k8s-version": "ci/latest",
  "num-both": "1",
  "num-controllers": "1",
  "num-workers": "1",
  "cloud-provider": "external",
  "e2e-focus": "should provide DNS for the cluster[[:space:]]{0,}\\[Conformance\\]",
  "e2e-skip": "Alpha|\\[(Disruptive|Feature:[^\\]]+|Flaky)\\]",
  "kube-conformance-image": "akutz/kube-conformance:latest",
  "config-env": "/config.env",
  "gcr-key-file": "/keyfile.json"
}
```

The file `terraform-output.txt` includes information about the cluster that was turned up:

```shell
controllers = [
    192.168.3.207
]
controllers-with-kubelets = 1
etcd = https://discovery.etcd.io/8c6a3f14571bf6892d370c325a428d9b
external_fqdn = sk8lb-ee7dff5-6b4c40f39f288cf1.elb.us-west-2.amazonaws.com
kubeconfig = data/prow-fb3ba63/kubeconfig
workers = [
    192.168.3.182
]
```

And finally, the files `e2e.log` and `junit_01.xml` are the log for the e2e  execution and the file parsed by the K8s test grid.

The remaining files are created by Sonobuoy during the test execution.
