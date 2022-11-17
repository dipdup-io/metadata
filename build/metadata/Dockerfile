# ---------------------------------------------------------------------
#  The first stage container, for building the application
# ---------------------------------------------------------------------
FROM golang:1.18-alpine as builder

ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOOS=linux

RUN apk --no-cache add ca-certificates
RUN apk add --update git musl-dev gcc build-base

RUN mkdir -p $GOPATH/src/github.com/dipdup-net/metadata/

COPY ./go.* $GOPATH/src/github.com/dipdup-net/metadata/
WORKDIR $GOPATH/src/github.com/dipdup-net/metadata
RUN go mod download

COPY cmd/metadata cmd/metadata
COPY internal internal

WORKDIR $GOPATH/src/github.com/dipdup-net/metadata/cmd/metadata/
RUN go build -a -o /go/bin/dipdup-metadata .

# ---------------------------------------------------------------------
#  The second stage container, for running the application
# ---------------------------------------------------------------------
FROM scratch

WORKDIR /app/metadata/

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/dipdup-metadata /go/bin/dipdup-metadata
COPY ./cmd/metadata/mappings ./mappings
COPY ./cmd/metadata/graphql ./graphql
COPY ./build/*.yml ./
COPY ./cmd/metadata/views/*.sql ./views/
COPY ./cmd/metadata/sql/*.sql ./sql/
COPY ./cmd/metadata/custom_hasura_config ./custom_hasura_config

ENTRYPOINT ["/go/bin/dipdup-metadata"]
