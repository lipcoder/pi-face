#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import time
import json
import csv
import hashlib
import logging
from logging.handlers import RotatingFileHandler
from datetime import datetime

import cv2
import inspireface as isf

from app.config import (
    FEATURE_DB_DIR,
    FEATURE_DB_PATH,
    LABEL_MAP_PATH,
    LOG_DIR,
    RECORDS_CSV_PATH,
    SEARCH_THRESHOLD,

    HIK_IP, HIK_USER, HIK_PWD, HIK_PORT,
    HIK_CHANNEL_MAIN, HIK_CHANNEL_SUB,

    RTSP_OPEN_TIMEOUT_SEC,
    RTSP_TEST_FRAMES,
    RTSP_MAX_CONSECUTIVE_FAILS,
    RTSP_READ_FAIL_SLEEP_SEC,
    RTSP_FREEZE_MAX_FRAMES,
)

os.makedirs(LOG_DIR, exist_ok=True)
LOG_FILE = os.path.join(LOG_DIR, "1.txt")

logger = logging.getLogger("face_runtime")
logger.setLevel(logging.INFO)
logger.propagate = False

if not logger.handlers:
    handler = RotatingFileHandler(
        LOG_FILE,
        maxBytes=20 * 1024 * 1024,
        backupCount=5,
        encoding="utf-8"
    )
    handler.setFormatter(logging.Formatter(
        fmt="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    ))
    logger.addHandler(handler)


# OpenCV / FFmpeg RTSP 设置
os.environ.setdefault(
    "OPENCV_FFMPEG_CAPTURE_OPTIONS",
    "rtsp_transport;tcp|stimeout;5000000|max_delay;500000"
)


# 全局状态
KNOWN_LABEL_MAP = {}
SEP_LINE = "=" * 59  # 日志分隔线长度固定，方便目视


def log_block(title: str):
    """
    以分隔块形式输出关键阶段日志
    """
    logger.info(SEP_LINE)
    logger.info(title)
    logger.info(SEP_LINE)


def load_label_map():
    global KNOWN_LABEL_MAP
    if os.path.exists(LABEL_MAP_PATH):
        try:
            with open(LABEL_MAP_PATH, "r", encoding="utf-8") as f:
                KNOWN_LABEL_MAP = json.load(f)
            logger.info("label_map loaded, entries=%d", len(KNOWN_LABEL_MAP))
        except Exception as e:
            KNOWN_LABEL_MAP = {}
            logger.error("failed to load label_map: %s", e)
    else:
        KNOWN_LABEL_MAP = {}
        logger.warning("label_map not found, only output identity_id")


def frame_hash(frame):
    return hashlib.md5(frame.tobytes()).hexdigest()


def rtsp_urls():
    base = f"rtsp://{HIK_USER}:{HIK_PWD}@{HIK_IP}:{HIK_PORT}"
    return [
        f"{base}/{HIK_CHANNEL_MAIN}?transportmode=unicast",
        f"{base}/{HIK_CHANNEL_SUB}?transportmode=unicast",
    ]


def try_open_stream(url):
    start = time.time()
    cap = cv2.VideoCapture(url, cv2.CAP_FFMPEG)
    if not cap.isOpened():
        cap.release()
        return None

    ok_count = 0
    while time.time() - start < RTSP_OPEN_TIMEOUT_SEC:
        ok, frame = cap.read()
        if ok and frame is not None and frame.size > 0:
            ok_count += 1
            if ok_count >= RTSP_TEST_FRAMES:
                return cap
        else:
            time.sleep(0.05)

    cap.release()
    return None


def log_to_csv(ts, label, confidence, status):
    with open(RECORDS_CSV_PATH, "a", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow([
            ts,
            label or "",
            f"{confidence:.6f}",
            f"{SEARCH_THRESHOLD:.6f}",
            status,
        ])


def init_inspireface():
    log_block("InspireFace initialization started")

    try:
        isf.reload("Pikachu")
        logger.info("InspireFace model reloaded successfully")
    except Exception as e:
        logger.warning("InspireFace reload failed: %s", e)

    session = isf.InspireFaceSession(
        isf.HF_ENABLE_FACE_RECOGNITION,
        isf.HF_DETECT_MODE_ALWAYS_DETECT,
    )
    session.set_detection_confidence_threshold(0.5)

    cfg = isf.FeatureHubConfiguration(
        primary_key_mode=isf.HF_PK_AUTO_INCREMENT,
        enable_persistence=True,
        persistence_db_path=FEATURE_DB_PATH,
        search_threshold=SEARCH_THRESHOLD,
        search_mode=isf.HF_SEARCH_MODE_EAGER,
    )
    isf.feature_hub_enable(cfg)

    logger.info(
        "FeatureHub enabled, face_count=%d",
        isf.feature_hub_get_face_count()
    )

    load_label_map()
    log_block("InspireFace initialization completed")
    return session


def main():
    log_block("face_runtime process starting")

    session = init_inspireface()

    urls = rtsp_urls()
    log_block("RTSP candidates configured")
    for u in urls:
        logger.info("RTSP candidate: %s", u)

    cap = None
    working_url = None

    consecutive_fails = 0
    last_hash = None
    freeze_count = 0
    first_frame_logged = False

    try:
        while True:
            # ---------- 连接 RTSP ----------
            if cap is None:
                for url in urls:
                    logger.info("trying RTSP: %s", url)
                    cap = try_open_stream(url)
                    if cap is not None:
                        working_url = url
                        log_block(f"RTSP connected successfully: {working_url}")
                        break
                if cap is None:
                    logger.error("all RTSP candidates failed, retrying")
                    time.sleep(1)
                    continue

            # ---------- 读取帧 ----------
            ok, frame = cap.read()
            if not ok or frame is None:
                consecutive_fails += 1
                if consecutive_fails >= RTSP_MAX_CONSECUTIVE_FAILS:
                    logger.error(
                        "RTSP read failed %d times, reconnecting: %s",
                        consecutive_fails, working_url
                    )
                    cap.release()
                    cap = None
                    consecutive_fails = 0
                time.sleep(RTSP_READ_FAIL_SLEEP_SEC)
                continue

            if not first_frame_logged:
                log_block(f"First frame received from RTSP: {working_url}")
                first_frame_logged = True

            consecutive_fails = 0

            # ---------- 冻结检测 ----------
            h = frame_hash(frame)
            if h == last_hash:
                freeze_count += 1
                if freeze_count >= RTSP_FREEZE_MAX_FRAMES:
                    log_block(
                        f"Frame frozen detected, reconnecting RTSP: {working_url}"
                    )
                    cap.release()
                    cap = None
                    freeze_count = 0
                    last_hash = None
                    continue
            else:
                freeze_count = 0
                last_hash = h

            # ---------- 人脸识别 ----------
            faces = session.face_detection(frame)
            if not faces:
                continue

            for face in faces:
                feature = session.face_feature_extract(frame, face)
                if feature is None or feature.size == 0:
                    continue

                result = isf.feature_hub_face_search(feature)
                ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

                if (
                    result
                    and result.similar_identity
                    and result.similar_identity.id != -1
                    and result.confidence >= SEARCH_THRESHOLD
                ):
                    identity_id = int(result.similar_identity.id)
                    label = KNOWN_LABEL_MAP.get(str(identity_id))
                    conf = float(result.confidence)

                    logger.info(
                        "MATCH id=%d label=%s conf=%.3f",
                        identity_id, label, conf
                    )
                    log_to_csv(ts, label, conf, "MATCH")
                else:
                    logger.info("UNKNOWN face detected")
                    log_to_csv(ts, "", 0.0, "UNKNOWN")

    except KeyboardInterrupt:
        log_block("face_runtime interrupted by user")

    except Exception as e:
        log_block("face_runtime fatal error")
        logger.critical("exception: %s", e, exc_info=True)

    finally:
        if cap is not None:
            cap.release()
        log_block("face_runtime process stopped")

# =========================================================

if __name__ == "__main__":
    main()
