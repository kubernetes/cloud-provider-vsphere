# Use a dedicated stage to download the project's dependencies.
FROM golang:{{GO_VERSION}} as builder
WORKDIR /build
COPY go.mod go.sum /build/
COPY cmd /build/cmd/
COPY pkg /build/pkg/
RUN go mod download && go mod verify

FROM golang:{{GO_VERSION}}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# Copy the Go module cache from the builder.
COPY --from=builder /go/pkg/mod \
                    /go/pkg/mod/