FROM golang:1.26.11
WORKDIR /go/src
# make deps fetching cacheable
COPY go.mod go.sum ./
RUN go mod download
# build
COPY . .
RUN make build

# build real cloud-provider-kind image
# only the docker CLI is needed, the engine is reached through the mounted docker socket
FROM docker:29.10.0-cli
# pick up Alpine security fixes (incl. ca-certificates >= 20261020-r0, eMudhra distrust)
RUN apk upgrade --no-cache
COPY --from=0 --chown=root:root ./go/src/bin/cloud-provider-kind /bin/cloud-provider-kind
ENTRYPOINT ["/bin/cloud-provider-kind"]
