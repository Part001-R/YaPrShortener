Генерация кода из файла proto.

protoc \
  --go_out=paths=source_relative:. \
  --go-grpc_out=paths=source_relative:. \
  proto/service/service.proto