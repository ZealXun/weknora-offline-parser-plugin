# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG GOPROXY_ARG=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY_ARG}

WORKDIR /workspace/WeKnora
COPY --from=weknora . .

WORKDIR /workspace/plugin
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/weknora-offline-parser-plugin .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/weknora-offline-parser-plugin /weknora-offline-parser-plugin
USER 65532:65532
ENTRYPOINT ["/weknora-offline-parser-plugin"]
