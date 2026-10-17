# ---- 前端构建 ----
FROM node:22-alpine AS web
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# ---- 后端构建（纯 Go，无 CGO）----
FROM golang:1.27-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/lmb666666/wisp/internal/version.Version=${VERSION} -X github.com/lmb666666/wisp/internal/version.Commit=${COMMIT}" \
    -o /out/wisp ./cmd/wisp

# ---- 运行 ----
FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="Wisp" \
      org.opencontainers.image.description="自托管的轻量说说 / 朋友圈" \
      org.opencontainers.image.source="https://github.com/lmb666666/wisp" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"
COPY --from=build /out/wisp /wisp
ENV WISP_DATA_DIR=/data \
    WISP_PORT=8080
VOLUME /data
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/wisp"]
