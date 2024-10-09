FROM alpine:edge AS builder
RUN apk update && apk add build-base

COPY . /app
WORKDIR /app

RUN cc -Wall -static -o /sysctl /app/sysctl.c
RUN strip --strip-all /sysctl

FROM scratch

COPY --from=builder /sysctl /usr/local/bin/sysctl

ENTRYPOINT ["/usr/local/bin/sysctl"]
