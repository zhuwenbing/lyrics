# 第一阶段：构建二进制文件
FROM --platform=$TARGETPLATFORM golang:alpine AS builder

LABEL maintainer="Mr. Chu"

# 设置工作目录
WORKDIR /app

# 将代码复制到容器中
COPY lyrics.go go.mod go.sum .

# 构建应用程序
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o lyrics .

# 第二阶段：生成最小化的镜像
FROM alpine

# 复制二进制文件
COPY --from=builder /app/lyrics /usr/local/bin/

# 更新系统
RUN set -eux; \
    apk --no-cache --no-progress --update upgrade; \
    rm -rf /var/cache/apk/*; \
    rm -rf /tmp/*

# 启动应用
CMD [ "lyrics" ]
