# pi-face Dockerfile - Ubuntu 24.04 + Python 3.12.7 + venv
# 放在项目根目录（和 app/、web/ 同级）

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    TZ=Asia/Shanghai

# ---------------------------------------------------------
# 1) 把 apt 源切到清华镜像（HTTP，避免证书问题）
#    - Ubuntu 24.04 使用 deb822 格式的 /etc/apt/sources.list.d/ubuntu.sources
#    - 同时兼容老的 /etc/apt/sources.list
# ---------------------------------------------------------
RUN set -eux; \
    if [ -f /etc/apt/sources.list ]; then \
      sed -i \
        's|http://archive.ubuntu.com/ubuntu|http://mirrors.tuna.tsinghua.edu.cn/ubuntu|g; \
         s|http://security.ubuntu.com/ubuntu|http://mirrors.tuna.tsinghua.edu.cn/ubuntu|g' \
        /etc/apt/sources.list; \
    fi; \
    if [ -f /etc/apt/sources.list.d/ubuntu.sources ]; then \
      sed -i \
        's|http://archive.ubuntu.com/ubuntu|http://mirrors.tuna.tsinghua.edu.cn/ubuntu|g; \
         s|http://security.ubuntu.com/ubuntu|http://mirrors.tuna.tsinghua.edu.cn/ubuntu|g' \
        /etc/apt/sources.list.d/ubuntu.sources; \
    fi

# ---------------------------------------------------------
# 2) 系统依赖：
#    - 构建 Python 3.12.7 需要的开发库
#    - OpenCV / InspireFace 运行时需要的图形/多媒体库
# ---------------------------------------------------------
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential \
      wget \
      curl \
      ca-certificates \
      libssl-dev \
      zlib1g-dev \
      libbz2-dev \
      libreadline-dev \
      libsqlite3-dev \
      libncurses-dev \
      libffi-dev \
      liblzma-dev \
      uuid-dev \
      libgdbm-dev \
      tk-dev \
      libglib2.0-0 \
      libgl1 \
      libsm6 \
      libxext6 \
      libxrender1 \
      ffmpeg \
      git && \
    rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------
# 3) 在 Ubuntu 上从源码编译安装 Python 3.12.7
#    优先用国内华为云镜像，失败再回退 python.org
# ---------------------------------------------------------
ENV PYTHON_VERSION=3.12.7

RUN set -eux; \
    cd /tmp; \
    CN_URL="https://mirrors.huaweicloud.com/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz"; \
    OFFICIAL_URL="https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz"; \
    echo "Downloading Python ${PYTHON_VERSION} from ${CN_URL}"; \
    if ! wget -O "Python-${PYTHON_VERSION}.tgz" "${CN_URL}"; then \
        echo "CN mirror failed, fallback to ${OFFICIAL_URL}"; \
        wget -O "Python-${PYTHON_VERSION}.tgz" "${OFFICIAL_URL}"; \
    fi; \
    tar -xzf "Python-${PYTHON_VERSION}.tgz"; \
    cd "Python-${PYTHON_VERSION}"; \
    ./configure --enable-optimizations --prefix=/opt/python; \
    make -j"$(nproc)"; \
    make install; \
    cd /; \
    rm -rf "/tmp/Python-${PYTHON_VERSION}"*

# 把 /opt/python 放到 PATH 前面，默认 python3 就是 3.12.7
ENV PATH="/opt/python/bin:${PATH}"

# ---------------------------------------------------------
# 4) 创建虚拟环境 + pip 换清华源
# ---------------------------------------------------------
RUN python3 -m ensurepip && \
    python3 -m pip install --no-cache-dir --upgrade pip && \
    python3 -m pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple && \
    python3 -m venv /opt/venv

ENV VIRTUAL_ENV=/opt/venv
ENV PATH="/opt/venv/bin:${PATH}"
ENV PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple

# ---------------------------------------------------------
# 5) 应用相关环境变量
#    默认用 /data，当作 DATA_ROOT，和 app/config.py 保持一致
# ---------------------------------------------------------
ENV DATA_ROOT=/data

# ---------------------------------------------------------
# 6) 拷贝项目代码
# ---------------------------------------------------------
WORKDIR /app
COPY . /app

# ---------------------------------------------------------
# 7) 安装 Python 依赖
#    - 有 requirements.txt 就按它来
#    - 没有的话兜底装关键依赖（包括 modelscope / loguru / tqdm）
# ---------------------------------------------------------
RUN if [ -f requirements.txt ]; then \
      pip install --no-cache-dir -r requirements.txt; \
    else \
      pip install --no-cache-dir inspireface opencv-python flask modelscope loguru tqdm; \
    fi

# ---------------------------------------------------------
# 8) 预创建 data 目录（实际运行时会被 volume 覆盖）
# ---------------------------------------------------------
RUN mkdir -p /data/know /data/unknow /data/feature_db /data/logs

# ---------------------------------------------------------
# 9) 入口脚本（同时起 hik_mjpeg_server + face_runtime）
# ---------------------------------------------------------
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# hik_mjpeg_server 的 HTTP 端口
EXPOSE 5000

ENTRYPOINT ["/entrypoint.sh"]
CMD []

