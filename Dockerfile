FROM --platform=$BUILDPLATFORM golang:1.26.11
WORKDIR /go/src
# make deps fetching cacheable
COPY go.mod go.sum ./
RUN go mod download
# build
COPY . .
ARG TARGETARCH
RUN GOARCH=$TARGETARCH make build

# build real cloud-provider-kind image
# only the docker CLI is needed, the engine is reached through the mounted docker socket
FROM docker:29.10.0-cli
# pick up Alpine security fixes (incl. ca-certificates >= 20261020-r0, eMudhra distrust)
RUN apk upgrade --no-cache
COPY --from=0 --chown=root:root ./go/src/bin/cloud-provider-kind /bin/cloud-provider-kind
# Docker Desktop: publish the Service ports on the host (no userspace tunnels), and do not
# install the Gateway API CRDs nor a default IngressClass in the cluster
ENTRYPOINT ["/bin/cloud-provider-kind", "--enable-lb-port-mapping", "--gateway-channel=disabled", "--enable-default-ingress=false"]
