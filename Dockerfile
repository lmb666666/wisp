# ---- 前端只构建一次（构建平台）----
# Node 版本与 CI（ci.yml setup-node）保持一致
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# ---- Go 交叉编译（不在 QEMU 下跑编译器）----
# Go 版本与 go.mod 的 go 指令保持一致
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -o /out/wisp-healthcheck ./cmd/wisp-healthcheck
RUN mkdir -p /out/data && touch /out/data/.keep

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
COPY --from=build /out/wisp-healthcheck /wisp-healthcheck
# 预创建 /data 并交给 nonroot；命名卷首次挂载会继承该目录权限，避免非 root 无法写库。
COPY --from=build --chown=65532:65532 /out/data /data
ENV WISP_DATA_DIR=/data \
    WISP_PORT=8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/wisp-healthcheck"]
USER nonroot
ENTRYPOINT ["/wisp"]
