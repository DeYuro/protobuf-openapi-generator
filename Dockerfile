FROM golang:1.17

# Common
RUN : \
    && apt-get update -y \
    && mkdir /input \
    && mkdir /generator

# Install protobuf compiler
RUN : \
    && apt-get install protobuf-compiler -y;

#  Copy asserts
COPY assets/include /usr/local/include
COPY assets/generator.go /go/src/generator/

# Install protoc-gen-openapi
RUN : \
    && go install github.com/google/gnostic/apps/protoc-gen-openapi@latest

# Install generator
RUN : \
    && cd src/generator \
    && go mod init \
    && go mod tidy \
    && go install


ENTRYPOINT ["generator"]