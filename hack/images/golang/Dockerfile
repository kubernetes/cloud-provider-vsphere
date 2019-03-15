ARG GO_VERSION=1.12

# The deps stage creates a module cache.
FROM golang:${GO_VERSION} as deps
WORKDIR /build
COPY pkg /build/pkg/
COPY cmd /build/cmd/
COPY go.mod go.sum /build/
RUN go mod download && go mod verify

# The build stage creates a build cache.
FROM golang:${GO_VERSION} as build
WORKDIR /build
COPY . ./
COPY --from=deps /go/pkg/mod /go/pkg/mod/

# The build cache is primed by building the CCM and CSI binaries. The GOOS and
# GOARCH values used are configurable when building this image. This supports
# images with different GOOS_GOARCH build caches.
ARG GOOS
ARG GOARCH
ENV GOOS=${GOOS:-linux} GOARCH=${GOARCH:-amd64}
RUN make build-ccm build-csi

# The main stage contains the module and build caches for subsequent builds.
FROM golang:${GO_VERSION}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# Copy the Go module cache into the main stage.
COPY --from=build /go/pkg/mod /go/pkg/mod/

# Copy the Go build cache into the main stage.
COPY --from=build /root/.cache/go-build /root/.cache/go-build/
