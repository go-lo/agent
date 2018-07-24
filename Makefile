default: clean build

.PHONY: build
build: clean loadtest-agent

loadtest-agent:
	CGO_ENABLED=0 GOOS=linux go build

.PHONY: clean
clean:
	-rm loadtest-agent

.PHONY: docker
docker:
	docker build -t jspc/loadtest-agent .
	docker push jspc/loadtest-agent
