# syntax=docker/dockerfile:1.7
# 官方 Go 镜像，自带完整工具链
FROM golang:1.25.0

WORKDIR /app

# 先复制依赖文件并下载依赖，使用 BuildKit 缓存长期复用模块下载
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod-cache \
    go mod download

# 复制所有项目文件
COPY . .

# 预编译，确认基础代码健康
RUN --mount=type=cache,target=/root/.cache/go-build,id=go-build-cache \
    go build ./...

# 容器启动后进入 shell，方便模型操作
CMD ["bash"]
