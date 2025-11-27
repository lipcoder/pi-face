# app/config.py
import os

# ========== 项目根路径 主要供我自己调试 ==========
BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# ========== 数据根目录 ==========
"""
DATA_ROOT 是所有数据的根目录：
- 已知人脸: {DATA_ROOT}/know
- 未知人脸: {DATA_ROOT}/unknow
- 特征库:   {DATA_ROOT}/feature_db
- 日志:     {DATA_ROOT}/logs/records.csv
"""

DATA_ROOT = os.environ.get("DATA_ROOT", os.path.join(BASE_DIR, "data"))
# 这个变量在docekr-compose里面声明为了/data所以不需要考虑，当前是本地当前目录，为了方便我运行，将括号内第二个变量换成”/data“即可调整

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

# 海康主码流 101
HIK_CHANNEL_MAIN = os.environ.get("HIK_CHANNEL_MAIN", "Streaming/Channels/101")
HIK_CHANNEL_SUB = os.environ.get("HIK_CHANNEL_SUB", "Streaming/Channels/102")

HTTP_HOST = os.environ.get("HTTP_HOST", "0.0.0.0")
HTTP_PORT = int(os.environ.get("HTTP_PORT", "5000"))
