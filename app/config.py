# app/config.py
# -*- coding: utf-8 -*-

import os

# 项目根目录（方便本地调试)
BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# os.path.abspath(__file__),abspath(...) 把路径转成绝对路径
# os.path.dirname(path) 取路径的目录部分，相当于去掉文件名
# os.path.dirname(...) 再对上一步结果取一次 dirname，就是再往上一级目录

# 数据根目录
# Docker 场景下建议在 docker-compose 中设置 DATA_ROOT=/data
DATA_ROOT = os.environ.get(
    "DATA_ROOT",
    os.path.join(BASE_DIR, "data")
)

# 特征库
FEATURE_DB_DIR = os.path.join(DATA_ROOT, "feature_db")
FEATURE_DB_PATH = os.path.join(FEATURE_DB_DIR, "feature_hub.db")
LABEL_MAP_PATH = os.path.join(FEATURE_DB_DIR, "label_map.json")

# 已知人脸 日志 & 识别记录
KNOW_DIR = os.path.join(DATA_ROOT, "know")
LOG_DIR = os.path.join(DATA_ROOT, "logs")
RECORDS_CSV_PATH = os.path.join(LOG_DIR, "records.csv")


# 海康参数
HIK_IP = os.environ.get("HIK_IP", "192.168.1.111")
HIK_USER = os.environ.get("HIK_USER", "admin")
HIK_PWD = os.environ.get("HIK_PWD", "Long-Live-NBALab")
HIK_PORT = int(os.environ.get("HIK_PORT", "554"))

# 主码流 / 子码流（顺序即优先级）
HIK_CHANNEL_MAIN = os.environ.get(
    "HIK_CHANNEL_MAIN",
    "Streaming/Channels/101"
)
HIK_CHANNEL_SUB = os.environ.get(
    "HIK_CHANNEL_SUB",
    "Streaming/Channels/102"
)


# =======================================================================================




# 特征搜索阈值（越大越严格）
SEARCH_THRESHOLD = float(
    os.environ.get("SEARCH_THRESHOLD", "0.48")
)

# 单个 RTSP URL 探测最大耗时（秒）
RTSP_OPEN_TIMEOUT_SEC = int(
    os.environ.get("RTSP_OPEN_TIMEOUT_SEC", "6")
)

# 探测阶段，必须成功读到的帧数
RTSP_TEST_FRAMES = int(
    os.environ.get("RTSP_TEST_FRAMES", "8")
)

# 连续读帧失败多少次 → 强制释放并重连
RTSP_MAX_CONSECUTIVE_FAILS = int(
    os.environ.get("RTSP_MAX_CONSECUTIVE_FAILS", "30")
)

# 单次读帧失败后的 sleep（秒）
RTSP_READ_FAIL_SLEEP_SEC = float(
    os.environ.get("RTSP_READ_FAIL_SLEEP_SEC", "0.2")
)

# 连续多少帧 hash 不变 → 判定为“冻结”
RTSP_FREEZE_MAX_FRAMES = int(
    os.environ.get("RTSP_FREEZE_MAX_FRAMES", "30")
)
