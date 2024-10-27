FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/go:latest AS builder

WORKDIR /app
COPY . .
WORKDIR /app/kv-store/cmd

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -ldflags "-s -w" -trimpath -o /kv-store

FROM scratch

WORKDIR /data

COPY --from=builder /kv-store /kv-store

CMD [ "/kv-store" ]
