FROM golang:1.17


# Common
RUN : \
    && apt-get update -y

# Install protobuf compiler
RUN : \
    && apt-get install protobuf-compiler -y


# Install protoc-gen-openapi
RUN : \
    && go install github.com/google/gnostic/apps/protoc-gen-openapi@latest \
    && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1

ADD assets/include /usr/local/include

COPY api /proto/api

ENTRYPOINT ["sleep","infinity"]