# app/config.py
import os

# ========== 项目根路径（目前用得不多，留着扩展）==========
BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# ========== 数据根目录 ==========
"""
DATA_ROOT 是所有数据的根目录：
- 已知人脸: {DATA_ROOT}/know
- 未知人脸: {DATA_ROOT}/unknow
- 特征库:   {DATA_ROOT}/feature_db
- 日志:     {DATA_ROOT}/logs/records.csv

默认用 /data，方便 Docker 直接挂载：
  docker run -v /宿主机/data:/data -e DATA_ROOT=/data ...
如果你本地想用项目里的 ./data，也可以：
  export DATA_ROOT=/absolute/path/to/project/face/data
"""
DATA_ROOT = os.environ.get("DATA_ROOT", "/data")

# ========== 各子目录 / 文件 ==========
FEATURE_DB_DIR = os.path.join(DATA_ROOT, "feature_db")
FEATURE_DB_PATH = os.path.join(FEATURE_DB_DIR, "feature_hub.db")
LABEL_MAP_PATH = os.path.join(FEATURE_DB_DIR, "label_map.json")

KNOW_DIR = os.path.join(DATA_ROOT, "know")
UNKNOW_DIR = os.path.join(DATA_ROOT, "unknow")

LOG_DIR = os.path.join(DATA_ROOT, "logs")
RECORDS_CSV_PATH = os.path.join(LOG_DIR, "records.csv")

# ========== 人脸识别相关 ==========
SEARCH_THRESHOLD = float(os.environ.get("SEARCH_THRESHOLD", "0.48"))

# 视频流来源（给 face_runtime 用），默认还是你现在用的这个地址
VIDEO_SOURCE = os.environ.get("VIDEO_SOURCE", "http://127.0.0.1:5000/video_feed")

# ========== 海康摄像头 & MJPEG HTTP ==========
HIK_IP = os.environ.get("HIK_IP", "192.168.1.111")
HIK_USER = os.environ.get("HIK_USER", "admin")
HIK_PWD = os.environ.get("HIK_PWD", "Long-Live-NBALab")
HIK_PORT = int(os.environ.get("HIK_PORT", "554"))

# 默认主码流 101，可以通过 HIK_CHANNEL 覆盖
HIK_CHANNEL_MAIN = os.environ.get("HIK_CHANNEL_MAIN", "Streaming/Channels/101")
HIK_CHANNEL_SUB = os.environ.get("HIK_CHANNEL_SUB", "Streaming/Channels/102")

HTTP_HOST = os.environ.get("HTTP_HOST", "0.0.0.0")
HTTP_PORT = int(os.environ.get("HTTP_PORT", "5000"))

# ========== Web 前端端口（Go）==========
WEB_PORT = int(os.environ.get("WEB_PORT", "8080"))
