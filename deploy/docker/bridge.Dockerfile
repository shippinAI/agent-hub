# syntax=docker/dockerfile:1
# devagotchi bridge image (for container-kind machines): Go cross-compiled on the build platform, version
# stamped like scripts/release.sh, distroless static nonroot runtime. Context = repo root. Docs: 38.
ARG GO_IMAGE=golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
WORKDIR /src/bridge
COPY bridge/go.mod bridge/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY bridge/ ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -buildid= -X devagotchi/bridge/buildinfo.Version=${VERSION} -X devagotchi/bridge/buildinfo.Commit=${COMMIT} -X devagotchi/bridge/buildinfo.Date=${DATE}" \
    -o /out/devagotchi ./cmd/devagotchi

FROM ${RUNTIME_IMAGE}
COPY --from=build /out/devagotchi /usr/local/bin/devagotchi
# distroless "nonroot" is uid/gid 65532; numeric so Kubernetes runAsNonRoot can verify it
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/devagotchi"]
