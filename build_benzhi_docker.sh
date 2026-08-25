#!/bin/bash
# 任意命令失败时立即退出
set -e

# 第一个参数是镜像名，未提供时使用 my-project
IMAGE_NAME=${1:-my-project}
# 第二个参数是目标平台，未提供时使用 linux/amd64
DOCKER_PLATFORM=${2:-linux/amd64}

# 使用评测专用 Dockerfile 构建镜像，避免使用项目自带 Dockerfile
DOCKER_BUILDKIT=1 docker build --platform $DOCKER_PLATFORM -f benzhi.Dockerfile -t $IMAGE_NAME .

# 输出中文构建结果和后续测试提示
echo ""
echo "✅ Docker 镜像 '$IMAGE_NAME' 构建成功！"
echo ""
echo "📋 后续测试步骤："
echo "  • 进入交互式容器：docker run -it $IMAGE_NAME:latest"
echo ""
