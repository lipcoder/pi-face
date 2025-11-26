# --- 用 Go 单独编译 web/main.go ---
FROM golang:1.22-alpine AS web_builder

WORKDIR /src
# 只复制 web 目录即可
COPY web ./web

WORKDIR /src/web
ENV GO111MODULE=off
RUN go build -o /src/web/server main.go

# --- 最终镜像：Python + 已编译好的 web/server ---
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

# 安装 OpenCV/FFmpeg 等依赖（按你的 requirements 为准）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg libsm6 libgl1 ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

# 先装 Python 依赖
COPY requirements.txt .
RUN pip install -r requirements.txt

# 再复制整个项目
COPY . .

# 把 Go 编译好的二进制拷进来
COPY --from=web_builder /src/web/server ./web/server

# 启动脚本（你刚刚创建的）
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# 数据根目录：/data（里面有 feature_db/ know/ unknow/ logs/）
ENV DATA_ROOT=/data

# face_runtime 默认从本机的 hik 服务拉 MJPEG 流
ENV VIDEO_SOURCE=http://127.0.0.1:5000/video_feed

# 导出端口：5000 (MJPEG) + 8080 (web)
EXPOSE 5000 8080

# 默认启动三个服务（在 entrypoint.sh 里）
CMD ["./entrypoint.sh"]
