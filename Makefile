.PHONY: proto build test

proto:
	protoc --go_out=. --go_opt=paths=source_relative proto/dmgn/v1/dmgn.proto

build:
	go build ./...

test:
	go test ./...
