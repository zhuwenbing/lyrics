ARG BUILDER=alpine
ARG RELEASE=scratch

# 第一阶段：构建二进制文件
FROM --platform=$TARGETPLATFORM golang:$BUILDER AS builder
# 设置工作目录
WORKDIR /app
# 将代码复制到容器中
COPY lyrics.go go.mod go.sum .
# 构建应用程序
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o lyrics .

# 第二阶段：基于 指定镜像 创建最终镜像
FROM $RELEASE
LABEL maintainer="Mr. Chu"
# 复制二进制文件
COPY --from=builder /app/lyrics /usr/local/bin/
# 启动应用
CMD [ "lyrics" ]
