#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import time
import json
import csv
from datetime import datetime

import cv2
import inspireface as isf

# ================== 路径 & 配置（迁移时主要改这里） ==================

# 当前脚本所在目录
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# 项目 data 目录（默认为 ../data）
DATA_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, "..", "data"))

# 视频流来源（默认本地 5000 端口）
VIDEO_SOURCE = os.environ.get("VIDEO_SOURCE", "http://127.0.0.1:5000/video_feed")

# 未知人脸保存目录
FACE_SAVE_DIR = os.path.join(DATA_DIR, "unknow")

# 特征数据库目录与文件（与建库脚本保持完全一致）
FEATURE_DB_DIR = os.path.join(DATA_DIR, "feature_db")
FEATURE_DB_PATH = os.path.join(FEATURE_DB_DIR, "feature_hub.db")
LABEL_MAP_PATH = os.path.join(FEATURE_DB_DIR, "label_map.json")

# 日志目录 & 文件
LOG_DIR = os.path.join(DATA_DIR, "logs")
RECORDS_CSV_PATH = os.path.join(LOG_DIR, "records.csv")

os.makedirs(FACE_SAVE_DIR, exist_ok=True)
os.makedirs(FEATURE_DB_DIR, exist_ok=True)
os.makedirs(LOG_DIR, exist_ok=True)

# 识别阈值（要与建库脚本一致）
SEARCH_THRESHOLD = 0.48

# 是否显示调试窗口
SHOW_WINDOW = False

# 全局：face_id -> label
KNOWN_LABEL_MAP = {}


# ================== label_map 工具函数 ==================

def load_label_map():
    global KNOWN_LABEL_MAP
    if os.path.exists(LABEL_MAP_PATH):
        try:
            with open(LABEL_MAP_PATH, "r", encoding="utf-8") as f:
                KNOWN_LABEL_MAP = json.load(f)
            print(f"[INFO] 已加载 label_map: {len(KNOWN_LABEL_MAP)} 条")
        except Exception as e:
            print("[WARN] 读取 label_map 失败：", e)
            KNOWN_LABEL_MAP = {}
    else:
        KNOWN_LABEL_MAP = {}
        print("[WARN] 未找到 label_map.json，将只输出 id，不输出名字。")


# ================== InspireFace 初始化（只加载已有特征） ==================

def init_inspireface():
    """
    初始化 InspireFace 会话 + FeatureHub。
    不再从 know 目录建库，只使用已有数据库和 label_map。
    """
    try:
        isf.reload("Pikachu")
    except Exception as e:
        print("[WARN] reload Pikachu 失败：", e)

    opt = isf.HF_ENABLE_FACE_RECOGNITION

    session = isf.InspireFaceSession(
        opt,
        isf.HF_DETECT_MODE_ALWAYS_DETECT,
    )

    session.set_detection_confidence_threshold(0.5)

    feature_hub_cfg = isf.FeatureHubConfiguration(
        primary_key_mode=isf.HF_PK_AUTO_INCREMENT,
        enable_persistence=True,
        persistence_db_path=FEATURE_DB_PATH,
        search_threshold=SEARCH_THRESHOLD,
        search_mode=isf.HF_SEARCH_MODE_EAGER,
    )
    ret = isf.feature_hub_enable(feature_hub_cfg)
    assert ret, "Failed to enable FeatureHub"

    print("[INFO] InspireFace 初始化完成，特征库：", FEATURE_DB_PATH)
    print("[INFO] 当前库中已有的人脸数：", isf.feature_hub_get_face_count())

    load_label_map()

    return session


# ================== 工具函数 ==================

def crop_face_from_frame(frame, face):
    """
    根据 InspireFace 的 face.location 从整帧中截取人脸子图
    """
    h, w = frame.shape[:2]
    x1, y1, x2, y2 = map(int, face.location)

    x1 = max(0, min(x1, w - 1))
    y1 = max(0, min(y1, h - 1))
    x2 = max(0, min(x2, w))
    y2 = max(0, min(y2, h))

    if x2 <= x1 or y2 <= y1:
        return None

    return frame[y1:y2, x1:x2].copy()


def save_face_image(face_img):
    """
    把截取的人脸图保存到 FACE_SAVE_DIR 下
    返回保存路径
    """
    ts = datetime.now().strftime("%Y%m%d_%H%M%S_%f")
    filename = f"{ts}.jpg"
    save_path = os.path.join(FACE_SAVE_DIR, filename)
    cv2.imwrite(save_path, face_img)
    return save_path


def recognize_face(session, frame, face):
    """
    对单张人脸进行识别：
    1. 用整帧 + face 做特征提取
    2. 用 FeatureHub 搜索最近的一个 ID
    3. 返回 (是否匹配, 置信度, identity_id, label)
    """
    feature = session.face_feature_extract(frame, face)
    if feature is None or feature.size == 0:
        return False, 0.0, -1, None

    result = isf.feature_hub_face_search(feature)
    if result is None:
        return False, 0.0, -1, None

    confidence = getattr(result, "confidence", 0.0)
    identity_id = getattr(result, "face_id", None)
    if identity_id is None:
        identity_id = getattr(result, "identity_id", -1)

    is_match = confidence >= SEARCH_THRESHOLD

    label = KNOWN_LABEL_MAP.get(str(identity_id))
    return is_match, float(confidence), int(identity_id), label


# ================== 记录到 CSV ==================

def log_to_csv(timestamp, image_path, label, confidence, status):
    """
    严格按格式写入：
    2025-11-22 10:03:48,/data/unknow/20251122_100348_139706.jpg,xue,0.721357,0.480000,MATCH,
    """

    # 如果你需要绝对路径前缀固定为 /data，可以在这里进行替换/拼接：
    # 例如：
    #   abs_path = os.path.abspath(image_path)
    #   # 或强制替换为 /data/unknow 前缀（按你实际部署来改）
    #   log_path = abs_path
    # 这里先直接使用传入的 image_path（通常是相对 DATA_DIR 的路径）
    log_path = image_path

    with open(RECORDS_CSV_PATH, "a", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow([
            timestamp,                      # 2025-11-22 10:03:48
            log_path,                       # /data/unknow/xxx.jpg 或其他路径
            label or "",                    # xue
            f"{confidence:.6f}",            # 0.721357
            f"{SEARCH_THRESHOLD:.6f}",      # 0.480000
            status,                         # MATCH / UNKNOWN
            "",                             # 最后一个空字段，对应末尾那个逗号
        ])


# ================== 主循环：拉流 + 识别 ==================

def main():
    session = init_inspireface()

    print("[INFO] 使用视频源：", VIDEO_SOURCE)
    cap = cv2.VideoCapture(VIDEO_SOURCE)

    if not cap.isOpened():
        print("[ERROR] 无法打开视频源：", VIDEO_SOURCE)
        return

    try:
        while True:
            ret, frame = cap.read()
            if not ret or frame is None:
                print("[WARN] 读取帧失败，稍后重试...")
                time.sleep(0.1)
                continue

            faces = session.face_detection(frame)
            if not faces:
                if SHOW_WINDOW:
                    cv2.imshow("Face Runtime", frame)
                    if cv2.waitKey(1) & 0xFF == ord('q'):
                        break
                continue

            for face in faces:
                face_img = crop_face_from_frame(frame, face)
                if face_img is None:
                    continue

                face_path = save_face_image(face_img)

                is_match, conf, identity_id, label = recognize_face(session, frame, face)

                ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                if is_match:
                    status = "MATCH"
                    name_part = label if label else f"id={identity_id}"
                    print(f"[{ts}] MATCH {name_part} id={identity_id} conf={conf:.3f} img={face_path}")
                else:
                    status = "UNKNOWN"
                    print(f"[{ts}] UNKNOWN id={identity_id} conf={conf:.3f} img={face_path}")

                # 写入 CSV 日志
                log_to_csv(
                    timestamp=ts,
                    image_path=face_path,
                    label=label,
                    confidence=conf,
                    status=status,
                )

                if SHOW_WINDOW:
                    x1, y1, x2, y2 = map(int, face.location)
                    cv2.rectangle(frame, (x1, y1), (x2, y2), (0, 255, 0), 2)
                    txt = label if label else f"id={identity_id}"
                    label_txt = f"{txt} {conf:.2f}"
                    cv2.putText(frame, label_txt, (x1, max(0, y1 - 5)),
                                cv2.FONT_HERSHEY_SIMPLEX, 0.5, (255, 255, 255), 1)

            if SHOW_WINDOW:
                cv2.imshow("Face Runtime", frame)
                if cv2.waitKey(1) & 0xFF == ord('q'):
                    break

    finally:
        cap.release()
        if SHOW_WINDOW:
            cv2.destroyAllWindows()
        print("[INFO] 资源已释放，程序退出。")


if __name__ == "__main__":
    main()
