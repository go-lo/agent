default: clean test build docker

.PHONY: build
build: clean loadtest-agent

loadtest-agent:
	CGO_ENABLED=0 GOOS=linux go build

.PHONY: clean
clean:
	-rm loadtest-agent

.PHONY: test
test: deps
	go test -v -covermode=count -coverprofile="./count.out" ./...

.PHONY: deps
deps:
	go get -u -v ./...

.PHONY: docker
docker:
	docker build -t jspc/loadtest-agent .
	docker push jspc/loadtest-agent
