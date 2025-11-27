# Pi-Face · 人脸识别签到系统

Pi-Face 是一个基于 **InspireFace**
的人脸识别签到小系统，搭配海康摄像头使用，提供：

- 实时人脸识别与签到记录\
- 抓拍图片落盘 + CSV 日志\
- 一个简单的 Web 看板查看记录和统计

> 人脸识别核心来自：<https://github.com/HyperInspire/InspireFace>

## 1. 整体架构概览

系统由三部分组成：

1. **Python 服务（app）**
    - `hik_mjpeg_server.py`：从海康摄像头拉 RTSP 流，转成 HTTP MJPEG +
        snapshot
    - `face_runtime.py`：从 MJPEG 视频流中实时做人脸识别，并写入 CSV
        日志
    - `build_feature_db.py`：从已知人脸目录构建/增量更新特征库
2. **Go Web 服务（web）**
    - `web/main.go`：读取 CSV 日志文件，提供 JSON API 和静态页面
    - `web/static/index.html`：Tailwind + Chart.js 的前端看板
3. **Docker**
    - `dockerfile`：多阶段构建 Go + Python
    - `entrypoint.sh`：同时启动三个服务

## 2. 数据目录结构

    /data
    ├── know/
    ├── unknow/
    ├── feature_db/
    │   ├── feature_hub.db
    │   └── label_map.json
    └── logs/
        └── records.csv

## 3. Python 端

### 3.1 配置（config.py）

支持环境变量覆盖，如 `DATA_ROOT`、RTSP
配置、`SEARCH_THRESHOLD`、`VIDEO_SOURCE` 等。

### 3.2 build_feature_db.py

- 从 `know/` 读取图片
- 文件名作为人名
- 增量更新特征库（已有的 label 自动跳过）
- 输出 `feature_hub.db` + `label_map.json`

### 3.3 face_runtime.py

- 从 MJPEG 视频流取帧
- 检测 → 特征提取 → 搜索最近邻
- 保存人脸截帧到 `unknow/`
- 写入 CSV 日志，包含：时间、图片路径、姓名、相似度、阈值、状态等
- 自动处理掉线、自动重连

### 3.4 hik_mjpeg_server.py

- 自动尝试多个海康 RTSP URL
- 后台线程拉流
- 提供 `/video_feed` / `/snapshot` 接口

## 4. Web 看板（Go）

### 4.1 API

- `/api/records`（搜索、筛选、分页、时间倒序）
- `/api/stats`\
    基于 MATCH 统计：
  - 每人每天有效签到次数
  - 每天有效访客人数
  - 每月每人来的天数（按天去重）
- `/image?path=...` 访问抓拍图片（带路径安全检查）

### 4.2 前端

Tailwind + Chart.js，无需构建。\
包含图表、统计面板、记录表格、详情弹窗。

## 5. 使用方法

### 5.1 Docker（推荐）

创建：

    docker compose up --build

建库：

    docker run --rm pi-face python -m app.build_feature_db

运行现有镜像：

    docker compose up -d

访问： - MJPEG：<http://localhost:5000/> -
看板：<http://localhost:8080/>