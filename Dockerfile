FROM alpine
MAINTAINER jspc<james@zero-internet.org.uk>

RUN apk add --update --no-cache ca-certificates && \
    rm -rf /var/cache/apk

ADD loadtest-agent /

ENTRYPOINT ["/loadtest-agent"]
