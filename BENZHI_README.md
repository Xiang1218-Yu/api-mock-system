# api-mock-system Docker 交付说明

## 项目概览
- 面向前后端开发团队的 API 协作平台：接口定义管理、智能 Mock 数据生成、多接口聚合代理、自动 OpenAPI 文档。功能需求见 `system.md`。
- Go module: `api-mock-system`

## 标准命令

```bash
go build ./...
go test ./...
```

## 实际启动入口

```bash
go run ./cmd/server
```

## Docker 构建

```bash
./build_benzhi_docker.sh api-mock-system-benzhi linux/amd64
docker run --rm -it api-mock-system-benzhi bash
```

## 环境

- 基础镜像: `golang:1.26.5`
- 依赖在镜像构建阶段预下载，容器内可直接执行 Go 构建和测试命令。
- 代码目录: `/app`
- 源码中检测到的服务端口: `10`, `16`, `8080`
