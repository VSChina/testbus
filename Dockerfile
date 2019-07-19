FROM golang:latest

ENV SRC_DIR=src/github.com/VSChina/testhub/ \
    GO111MODULE=on
ADD . $SRC_DIR
RUN rm -rf $SRC_DIR/bin $SRC_DIR/output
WORKDIR $SRC_DIR/
RUN make

#FROM golang:latest
#WORKDIR /
#COPY --from=0 /go/src/github.com/VSChina/testhub/bin/testhub .