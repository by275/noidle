ARG ALPINE_VER=3.23
ARG GO_VERSION=1.26.4

FROM alpine:${ALPINE_VER} AS alpine
FROM golang:${GO_VERSION}-alpine${ALPINE_VER} AS golang
RUN go install github.com/by275/noidle@latest

FROM alpine
LABEL maintainer="by275"
LABEL org.opencontainers.image.source=https://github.com/by275/noidle

RUN apk add --no-cache tini
COPY --from=golang /go/bin/noidle /usr/local/bin/

# environment settings
ENV LANG=C.UTF-8 \
    PS1="\u@\h:\w\\$ "

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/noidle"]
