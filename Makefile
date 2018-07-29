default: clean test build docker

.PHONY: build
build: clean agent

agent:
	CGO_ENABLED=0 GOOS=linux go build

.PHONY: clean
clean:
	-rm agent

.PHONY: test
test: deps
	go test -v -covermode=count -coverprofile="./count.out" ./...

.PHONY: deps
deps:
	go get -u -v ./...

.PHONY: docker
docker:
	docker build -t goload/agent .
	docker push goload/agent
