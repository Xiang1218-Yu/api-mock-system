# api-mock-system

基于 Go 实现的API 接口聚合与 Mock 服务平台 Web 项目，一款后端服务，面向前后端开发团队的 API 协作平台：接口定义管理、智能 Mock 数据生成、多接口聚合代理、自动 OpenAPI 文档，功能需求见 `system.md`。

项目源代码、依赖描述和评测专用 Docker 文件共同构成自包含任务；不依赖本机预编译二进制。

## 标准构建、运行和测试命令

```bash
go build ./...
go run ./cmd/server
go test ./...
```
## 评测容器

评测专用 Dockerfile 为 `benzhi.Dockerfile`，构建脚本为 `build_benzhi_docker.sh`。

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh my-go-task linux/arm64
./build_benzhi_docker.sh my-go-task linux/amd64
docker run -it my-go-task:latest
```
