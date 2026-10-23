.PHONY: build test vet generate generate-query generate-proto tools

build:
	go build -trimpath -o email_sender .

test:
	go test ./...

vet:
	go vet ./...

generate: generate-query generate-proto

generate-query:
	go generate ./cmd/generate

generate-proto:
	PATH="$(CURDIR)/.bin:$$PATH" protoc -I internal/proto --go_out=paths=source_relative:api/grpc/emailsendpb --go-grpc_out=paths=source_relative:api/grpc/emailsendpb emailsend.proto

tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
