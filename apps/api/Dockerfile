# syntax=docker/dockerfile:1.7
# ReliefMesh API image: static Go binary on a distroless, non-root base.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=0.1.0
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w -X github.com/6-slx-6/reliefmesh/apps/api/internal/health.Version=${VERSION}" \
    -o /out/reliefmesh-api ./cmd/reliefmesh-api

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="reliefmesh-api" \
      org.opencontainers.image.description="ReliefMesh API - offline-first coordination for local aid and preparedness exercises" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/6-SlX-6/reliefmesh"
COPY --from=build /out/reliefmesh-api /usr/local/bin/reliefmesh-api
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/usr/local/bin/reliefmesh-api", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/reliefmesh-api"]
CMD ["serve"]
