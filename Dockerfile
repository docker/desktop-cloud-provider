FROM golang:1.26.10
WORKDIR /go/src
# make deps fetching cacheable
COPY go.mod go.sum ./
RUN go mod download
# build
COPY . .
RUN make build

# build real cloud-provider-kind image
FROM docker:29.9.2
# pick up ca-certificates >= 20261020-r0 (distrusts compromised eMudhra intermediates)
RUN apk upgrade --no-cache ca-certificates ca-certificates-bundle
COPY --from=0 --chown=root:root ./go/src/bin/cloud-provider-kind /bin/cloud-provider-kind
ENTRYPOINT ["/bin/cloud-provider-kind"]
