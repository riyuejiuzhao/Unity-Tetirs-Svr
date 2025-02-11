# 第一阶段：构建Go二进制（使用官方轻量级Alpine镜像）
FROM golang:1.22 AS builder

# 设置容器内工作目录
WORKDIR /app

# 先单独复制依赖文件以利用Docker缓存
COPY go.mod go.sum ./

# 下载依赖（使用国内镜像可加速）
RUN go mod tidy

# 复制所有源代码
COPY . .

# 编译静态二进制文件（禁用CGO）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main .

# 第二阶段：构建最小运行时镜像
FROM alpine:latest

# 从构建阶段复制编译结果
COPY --from=builder /app/main /app/main

# 设置工作目录并切换用户
WORKDIR /app

# 暴露服务端口
EXPOSE 50051

# 启动应用程序
CMD ["/app/main"]