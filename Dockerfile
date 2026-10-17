# ---- 前端只构建一次（构建平台）----
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# ---- Go 交叉编译（不在 QEMU 下跑编译器）----
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/lmb666666/wisp/internal/version.Version=${VERSION} -X github.com/lmb666666/wisp/internal/version.Commit=${COMMIT}" \
    -o /out/wisp ./cmd/wisp

# ---- 目标平台运行镜像 ----
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
