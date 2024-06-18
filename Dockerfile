FROM golang:1.22 as build
WORKDIR /go/src
# make deps fetching cacheable
COPY go.mod go.sum ./
RUN go mod download
# build
COPY . .
RUN make build

# build real cloud-provider-kind image
FROM docker:26
COPY --from=build --chown=root:root ./go/src/bin/cloud-provider-kind /bin/cloud-provider-kind
ENTRYPOINT ["/bin/cloud-provider-kind"]
