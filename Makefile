default: test

.PHONY: test
test:
	go test -v -covermode=count -coverprofile="./count.out" ./...

agent/:
	mkdir -p agent/

agent/agent.pb.go: agent/
	protoc -I protos/ protos/agent.proto --go_out=plugins=grpc:agent/
