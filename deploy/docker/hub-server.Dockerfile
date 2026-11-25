FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY bridge/ bridge/
COPY server/ server/
COPY --from=web /src/server/internal/httpapi/static/ server/internal/httpapi/static/
WORKDIR /src/server
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" \
    -o /out/hub-server ./cmd/hub-server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 hub \
    && mkdir -p /data \
    && chown hub:hub /data
USER hub
WORKDIR /data
COPY --from=build /out/hub-server /usr/local/bin/hub-server
ENTRYPOINT ["/usr/local/bin/hub-server"]
