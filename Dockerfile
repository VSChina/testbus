FROM golang:1.12.7-alpine3.10 AS build-dev
ENV SRC_DIR=src/github.com/VSChina/testhub/ \
    GO111MODULE=on
ADD . $SRC_DIR
RUN apk update && apk add --no-cache git make
RUN rm -rf $SRC_DIR/bin $SRC_DIR/output
WORKDIR $SRC_DIR/
RUN make

FROM alpine:3.10
WORKDIR /
RUN apk add ca-certificates
COPY --from=build-dev /go/src/github.com/VSChina/testhub .
