# =========================================================
# Base image
# =========================================================
FROM python:3.10-bullseye

# =========================================================
# Environment
# =========================================================
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    DATA_ROOT=/data \
    TZ=Asia/Shanghai

# =========================================================
# Replace Debian sources with Tsinghua mirror
# =========================================================
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list \
 && sed -i 's|security.debian.org|mirrors.tuna.tsinghua.edu.cn/debian-security|g' /etc/apt/sources.list

# =========================================================
# System dependencies + timezone
# =========================================================
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    curl \
    ca-certificates \
    ffmpeg \
    libglib2.0-0 \
    libgl1 \
    binutils \
    tzdata \
 && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
 && echo "Asia/Shanghai" > /etc/timezone \
 && rm -rf /var/lib/apt/lists/*

# =========================================================
# pip mirror
# =========================================================
RUN python -m pip install --upgrade pip \
 && pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple

# =========================================================
# Python dependencies
# =========================================================
RUN pip install \
    numpy \
    opencv-python \
    flask \
    inspireface

# =========================================================
# Workdir & code
# =========================================================
WORKDIR /workspace
COPY app ./app
RUN mkdir -p /data

# =========================================================
# Entrypoint
# =========================================================
CMD ["python3", "-m", "app.face_runtime"]
