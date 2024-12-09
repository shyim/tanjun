FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/go:latest AS builder

WORKDIR /app
COPY . .
WORKDIR /app/scheduler

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -ldflags "-s -w" -trimpath -o /scheduler

FROM scratch

WORKDIR /data

COPY --from=builder /scheduler /scheduler

CMD [ "/scheduler", "server" ]
