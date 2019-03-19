################################################################################
##                               BUILD ARGS                                   ##
################################################################################
# The golang image is used to create the project's module and build caches
# and is also the image on which this image is based.
ARG GOLANG_IMAGE=golang:1.12

################################################################################
##                            GO MOD CACHE STAGE                              ##
################################################################################
# Create a Go module cache.
FROM ${GOLANG_IMAGE} as mod-cache
WORKDIR /build
COPY go.mod go.sum ./
COPY pkg ./pkg/
COPY cmd ./cmd/
RUN go mod download && go mod verify

################################################################################
##                           GO BUILD CACHE STAGE                             ##
################################################################################
# Create a Go build cache.
FROM ${GOLANG_IMAGE} as build-cache
ENV VERSION=null
WORKDIR /build
COPY --from=mod-cache /go/pkg/mod /go/pkg/mod/
COPY Makefile go.mod go.sum ./
COPY pkg ./pkg/
COPY cmd ./cmd/
RUN make build-ccm build-csi

################################################################################
##                               KIND STAGE                                   ##
################################################################################
FROM ${GOLANG_IMAGE} as kind
RUN go get -u sigs.k8s.io/kind

################################################################################
##                               LINT STAGE                                   ##
################################################################################
FROM ${GOLANG_IMAGE} as lint
RUN go get -u golang.org/x/lint/golint

################################################################################
##                               MAIN STAGE                                   ##
################################################################################
FROM ${GOLANG_IMAGE}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

################################################################################
##                            CONFIGURE LOCALE                                ##
################################################################################
# Set the locale so that the gist command is happy.
ENV LANG=en_US.UTF-8 LC_ALL=C.UTF-8

################################################################################
##                             PACKAGE UPDATES                                ##
################################################################################
# Install the dependencies. The list is a uniion of the dependencies required
# by the following images:
#   * https://github.com/vmware/simple-k8s-test-env/blob/master/e2e/Dockerfile
#   * https://github.com/kubernetes/test-infra/blob/master/images/bootstrap/Dockerfile
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      jq \
      locales \
      mercurial \
      python3 \
      ruby \
      tar \
      unzip \
      zip && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/* && \
    curl -sSL https://bootstrap.pypa.io/get-pip.py | python3 - && \
    pip3 install setuptools wheel awscli --upgrade && \
    gem install gist

################################################################################
##                             DOCKER-IN-DOCKER                               ##
################################################################################
# Again, copied from test-infra's bootstrap image:
# https://github.com/kubernetes/test-infra/blob/master/images/bootstrap/Dockerfile

# Install Docker deps, some of these are already installed in the image but
# that's fine since they won't re-install and we can reuse the code below
# for another image someday.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      apt-transport-https \
      ca-certificates \
      curl \
      gnupg2 \
      software-properties-common \
      lsb-release && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/*

# Add the Docker apt-repository
RUN curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "${ID}")/gpg | apt-key add - && \
    add-apt-repository \
      "deb [arch=amd64] https://download.docker.com/linux/$(. /etc/os-release; echo "${ID}") \
      $(lsb_release -cs) stable"

# Install Docker
# TODO(bentheelder): the `sed` is a bit of a hack, look into alternatives.
# Why this exists: `docker service start` on debian runs a `cgroupfs_mount` method,
# We're already inside docker though so we can be sure these are already mounted.
# Trying to remount these makes for a very noisy error block in the beginning of
# the pod logs, so we just comment out the call to it... :shrug:
# TODO(benthelder): update docker version. This is pinned because of
# https://github.com/kubernetes/test-infra/issues/6187
RUN apt-get update && \
    apt-get install -y --no-install-recommends docker-ce=18.06.* && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/* && \
    sed -i 's/cgroupfs_mount$/#cgroupfs_mount\n/' /etc/init.d/docker

# Move Docker's storage location
RUN echo 'DOCKER_OPTS="${DOCKER_OPTS} --data-root=/docker-graph"' | \
    tee --append /etc/default/docker

# NOTE this should be mounted and persisted as a volume ideally (!)
# We will make a fallback one now just in case
RUN mkdir /docker-graph

# Setting this environment variable is an easy way for processes running
# in the container to know DinD is enabled.
ENV DOCKER_IN_DOCKER_ENABLED=true

################################################################################
##                          COPY FROM SK8E2E                                  ##
################################################################################
# TODO(akutz) File a patch with Docker so NAME in "COPY --from=NAME" can
#             accept an ARG value.
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /tf                       /tf/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /google-cloud-sdk         /google-cloud-sdk/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /usr/local/bin/govc       /usr/local/bin/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /usr/local/bin/keepalive  /usr/local/bin/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /usr/local/bin/kubectl    /usr/local/bin/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /usr/local/bin/sonobuoy   /usr/local/bin/
COPY --from=gcr.io/kubernetes-conformance-testing/sk8e2e:v0.2.1-8-g2f2a8de /usr/local/bin/terraform  /usr/local/bin/

################################################################################
##                       CONFIGURE GOOGLE CLOUD SDK                           ##
################################################################################
# Update the PATH to include the Google Cloud SDK and disable its prompts and
# update the gcloud components.
ENV PATH="/google-cloud-sdk/bin:${PATH}" CLOUDSDK_CORE_DISABLE_PROMPTS=1
RUN /google-cloud-sdk/bin/gcloud components update

################################################################################
##                             INSTALL KIND                                   ##
################################################################################
COPY --from=kind /go/bin/kind /usr/local/bin/

################################################################################
##                             INSTALL LINT                                   ##
################################################################################
COPY --from=lint /go/bin/golint /usr/local/bin/

################################################################################
##                         PRIME GO MOD & BUILD CACHES                        ##
################################################################################
COPY --from=mod-cache   /go/pkg/mod           /go/pkg/mod/
COPY --from=build-cache /root/.cache/go-build /root/.cache/go-build/

################################################################################
##                           ADD LOCAL SOURCES                                ##
################################################################################
# Copy the sources into the project's traditional Gopath location in the
# image. It's possible to bind mount up-to-date sources over the ones in
# the image when the latter is run as a container.
WORKDIR /go/src/k8s.io/cloud-provider-vsphere/
COPY . ./
