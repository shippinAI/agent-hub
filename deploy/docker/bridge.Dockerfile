FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src/bridge
COPY bridge/go.mod bridge/go.sum ./
RUN go mod download
COPY bridge/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X devagotchi/bridge/buildinfo.Version=$VERSION -X devagotchi/bridge/buildinfo.Commit=$COMMIT" \
    -o /out/devagotchi-bridge ./cmd/devagotchi-bridge

FROM alpine:3.23
RUN apk add --no-cache ca-certificates bubblewrap \
    && adduser -D -u 10001 bridge
USER bridge
WORKDIR /home/bridge
COPY --from=build /out/devagotchi-bridge /usr/local/bin/devagotchi-bridge
ENTRYPOINT ["/usr/local/bin/devagotchi-bridge"]
