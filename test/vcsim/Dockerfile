# Use a dedicated stage to build the vCenter simulator and govc binaries.
FROM golang:1.11.5 as builder
WORKDIR /go/src
ENV VCSIM_VERSION=v0.20.0
RUN go get -d github.com/vmware/govmomi && \
    git -C github.com/vmware/govmomi checkout -b build "${VCSIM_VERSION}" && \
    make -C github.com/vmware/govmomi/govc && \
    make -C github.com/vmware/govmomi/vcsim

FROM debian:stretch-20190204-slim
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# Update the CA certificates and clean up the apt cache.
RUN apt-get -y update && \
    apt-get -y --no-install-recommends install \
    ca-certificates curl iproute2 locales tar unzip && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/*

# Set the locale so that the gist command is happy.
ENV LANG=en_US.UTF-8
ENV LC_ALL=C.UTF-8

# Copy the vCenter simulator and govc binaries.
COPY --from=builder /go/src/github.com/vmware/govmomi/govc/govc \
                    /go/src/github.com/vmware/govmomi/vcsim/vcsim \
                    /usr/local/bin/

# Set the working directory.
WORKDIR /

# Copy the entrypoint script into the image.
COPY entrypoint.sh /
RUN chmod 0755 /entrypoint.sh

ENV GOVC_URL=https://localhost:8443/sdk
ENV GOVC_USERNAME=user
ENV GOVC_PASSWORD=pass
ENV GOVC_INSECURE="true"
ENV GOVC_DATACENTER="/DC0"
ENV GOVC_RESOURCE_POOL="/DC0/host/DC0_C0/Resources"
ENV GOVC_DATASTORE="/DC0/datastore/LocalDS_0"
ENV GOVC_FOLDER="/DC0/vm"
ENV GOVC_NETWORK="/DC0/network/VM Network"

# The default argument for the entrypoint will drop the user into a shell.
CMD [ "/usr/local/bin/vcsim", "-l", ":8443" ]
ENTRYPOINT [ "/entrypoint.sh" ]
