# 构建阶段：处理插件和元数据
ARG PYTHON_IMAGE=higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/python:3.11-alpine
ARG ALPINE_MIRROR=""

FROM $PYTHON_IMAGE AS builder

# 配置 Alpine 镜像源（可选，本地构建时可指定国内镜像加速）
ARG ALPINE_MIRROR
RUN if [ -n "$ALPINE_MIRROR" ]; then \
        sed -i "s|dl-cdn.alpinelinux.org|$ALPINE_MIRROR|g" /etc/apk/repositories; \
    fi

# 安装系统依赖
RUN apk add --no-cache \
    wget \
    ca-certificates \
    && update-ca-certificates

# 安装 ORAS 客户端
RUN set -eux; \
    ORAS_VERSION="1.2.3"; \
    ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
    wget -O /tmp/oras.tar.gz "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_$(echo ${ORAS_VERSION})_linux_${ARCH}.tar.gz" \
    && tar -zxvf /tmp/oras.tar.gz -C /usr/local/bin \
    && rm -rf /tmp/oras.tar.gz oras \
    && oras version

# 创建工作目录
WORKDIR /workspace

# 复制脚本
COPY pull_plugins.py plugins.properties ./

# 执行构建操作
RUN python3 pull_plugins.py --download-v2

# 运行阶段：最终镜像
FROM $PYTHON_IMAGE

# 从构建阶段复制生成的文件
COPY --from=builder /workspace/plugins /usr/share/plugin-server/plugins

# 复制上传/下载服务
COPY plugin_server.py /usr/local/bin/plugin_server.py

# 暴露端口
EXPOSE 8080

# 启动插件服务
CMD ["python3", "/usr/local/bin/plugin_server.py"]
