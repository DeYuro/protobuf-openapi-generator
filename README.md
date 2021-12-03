Protobuf -> OPENAPIV3 generator


docker build --tag generator .
docker run -d -v $PWD/api:/shared/api --name generator generator