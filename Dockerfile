ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION} AS builder

# 可配置主包路径与二进制名
ARG MAIN_PACKAGE=.
ARG BINARY_NAME=app
ARG VERSION=dev

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG CGOENABLED=0

WORKDIR /src

RUN go env -w GOPROXY=https://goproxy.cn,direct

# 拷贝依赖清单以利用缓存
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# 拷贝源码并构建
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=${CGOENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
	-ldflags "-s -w -X github.com/miebyte/goutils/buildinfo.Version=${VERSION} -X github.com/miebyte/goutils/buildinfo.ServiceName=${BINARY_NAME}" \
	-o /out/${BINARY_NAME} ${MAIN_PACKAGE}

# ========== Runtime ==========
FROM alpine:3.20

# 安装 CA 证书与时区数据
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

ARG BINARY_NAME=app
COPY --from=builder /out/${BINARY_NAME} /app/${BINARY_NAME}

USER app