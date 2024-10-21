MODULE=github.com/afshin-deriv/playground
PROTO_FILE=proto/v1/playground.proto

.PHONY: proto
proto:
	protoc --go_out=. --go-grpc_out=. --go_opt=module=$(MODULE) --go-grpc_opt=module=$(MODULE) $(PROTO_FILE)

.PHONY: build
build: proto
	go build -o bin/playground ./cmd/server/main.go

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: fmt
fmt:
	go fmt ./...

