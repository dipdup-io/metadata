# ---------------------------------------------------------------------
#  The first stage container, for building the application
# ---------------------------------------------------------------------
FROM golang:1.15-alpine as builder

ENV CGO_ENABLED=1
ENV GO111MODULE=on
ENV GOOS=linux
ENV CONFIG=dipdup.yml

RUN apk --no-cache add ca-certificates
RUN apk add --update git musl-dev gcc build-base

RUN mkdir -p $GOPATH/src/github.com/dipdup-net/metadata/

COPY ./go.* $GOPATH/src/github.com/dipdup-net/metadata/
WORKDIR $GOPATH/src/github.com/dipdup-net/metadata
RUN go mod download

COPY cmd/api cmd/api

WORKDIR $GOPATH/src/github.com/dipdup-net/metadata/cmd/api/
RUN go build -ldflags "-linkmode external -extldflags '-static' -s -w"  -a -installsuffix cgo -o /go/bin/dipdup-metadata-api .

# ---------------------------------------------------------------------
#  The second stage container, for running the application
# ---------------------------------------------------------------------
FROM scratch

WORKDIR /app/metadata-api

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/dipdup-metadata-api /go/bin/dipdup-metadata-api
COPY ./build/*.yml ./dipdup.yml

ENTRYPOINT ["/go/bin/dipdup-metadata-api"]
