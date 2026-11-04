# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

################################################################################
##                               BUILD ARGS                                   ##
################################################################################
# Pin the multi-platform image indexes while keeping their tags readable.
# The Docker Dependabot entry updates these references.
ARG GOLANG_IMAGE=golang:1.26.10@sha256:670bb5aa5349ae03b8516764718486f4f16399134bdc9acd0c46a23f043ab05c

# Distroless currently maintains Debian 13 images; Debian 11 tags are deprecated.
ARG DISTROLESS_IMAGE=gcr.io/distroless/static-debian13:latest@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f

################################################################################
##                              BUILD STAGE                                   ##
################################################################################
# Build the manager as a statically compiled binary so it has no dependencies
# libc, muscl, etc.
FROM ${GOLANG_IMAGE} as builder

# This build arg is the version to embed in the CPI binary
ARG VERSION=1.37.0

# This build arg controls the GOPROXY setting
ARG GOPROXY

WORKDIR /build
COPY go.mod go.sum ./
COPY pkg/    pkg/
COPY cmd/    cmd/
ENV CGO_ENABLED=0
ENV GOPROXY ${GOPROXY:-https://proxy.golang.org}
RUN go build -a -ldflags="-w -s -extldflags=static -X main.version=${VERSION}" -o vsphere-cloud-controller-manager ./cmd/vsphere-cloud-controller-manager

################################################################################
##                               MAIN STAGE                                   ##
################################################################################
# Copy the manager into the distroless image.
FROM ${DISTROLESS_IMAGE}
LABEL maintainer="Danqing Hou <danqing.hou@broadcom.com>"

USER nobody

COPY --from=builder /build/vsphere-cloud-controller-manager /bin/vsphere-cloud-controller-manager
ENTRYPOINT [ "/bin/vsphere-cloud-controller-manager" ]
