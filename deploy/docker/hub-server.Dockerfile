# syntax=docker/dockerfile:1
# hub-server image: web UI built on the build platform, Go cross-compiled (no QEMU), distroless static nonroot
# runtime (no shell, no package manager). Context = repo root. Docs: docs/devagotchi/38-ci-containers.md.
# Base images are pinned by digest; Dependabot (docker ecosystem) bumps tag+digest together.
ARG GO_IMAGE=golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
ARG NODE_IMAGE=node:24-trixie-slim@sha256:7f486cabe892bff8fef04c0f7da8dba84a560be3e4e2740b7a86afc57ebec315
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY web/ ./
RUN npx vite build --outDir /out/static --emptyOutDir

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/server
COPY bridge/go.mod bridge/go.sum /src/bridge/
COPY server/go.mod server/go.sum /src/server/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY bridge/ /src/bridge/
COPY server/ /src/server/
COPY --from=web /out/static /src/server/internal/httpapi/static
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -buildid=" -o /out/hub-server ./cmd/hub-server && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -buildid=" -o /out/hub ./cmd/hub

FROM ${RUNTIME_IMAGE}
COPY --from=build /out/hub-server /out/hub /usr/local/bin/
# Listen on all interfaces inside the container; every other knob stays an env var (HUB_DATABASE_URL_FILE, ...).
ENV HUB_LISTEN=0.0.0.0:8088
EXPOSE 8088
# distroless "nonroot" is uid/gid 65532; numeric so Kubernetes runAsNonRoot can verify it
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/hub-server"]
