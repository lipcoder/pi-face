#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import time
import json
import csv
from datetime import datetime

import cv2
import inspireface as isf

# ================== 路径 & 配置（统一走 app.config） ==================
from app.config import (
    FEATURE_DB_DIR,
    FEATURE_DB_PATH,
    LABEL_MAP_PATH,
    LOG_DIR,
    RECORDS_CSV_PATH,
    SEARCH_THRESHOLD,
    VIDEO_SOURCE,
)

# 确保目录存在
os.makedirs(FEATURE_DB_DIR, exist_ok=True)
os.makedirs(LOG_DIR, exist_ok=True)

# 是否显示调试窗口
SHOW_WINDOW = False

# 连续读取失败多少次后重建连接
MAX_FRAME_FAILS = 5

# ================== 性能开关（按需调整） ==================
# 仅每 N 帧做一次检测 + 识别；其余帧直接跳过处理（不做“跟踪补帧”）
DETECT_EVERY_N_FRAMES = 5  # 1=每帧都跑；建议 3~10 之间试
# 是否写 CSV（仍然保留识别记录）；不写盘图片
ENABLE_CSV_LOG = True

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

    return frame[y1:y2, x1:x2]


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

    # 关键：从 similar_identity.id 拿 ID
    if result.similar_identity is None or result.similar_identity.id == -1:
        return False, float(result.confidence), -1, None

    confidence = float(result.confidence)
    identity_id = int(result.similar_identity.id)

    is_match = confidence >= SEARCH_THRESHOLD
    label = KNOWN_LABEL_MAP.get(str(identity_id))

    return is_match, confidence, identity_id, label


# ================== 记录到 CSV ==================

def log_to_csv(timestamp, label, confidence, status):
    """
    写入 CSV（不再落盘图片，因此 image_path 为空）。
    """
    if not ENABLE_CSV_LOG:
        return

    with open(RECORDS_CSV_PATH, "a", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow([
            timestamp,
            "",  # image_path 留空
            label or "",
            f"{confidence:.6f}",
            f"{SEARCH_THRESHOLD:.6f}",
            status,
            "",
        ])


# ================== 主循环：拉流 + 识别（带自动重连） ==================

def main():
    session = init_inspireface()

    cap = None
    fail_count = 0
    frame_idx = 0

    try:
        while True:
            # 如果还没有 cap，或者 cap 被释放了 / 打不开，就尝试重新连接
            if cap is None or not cap.isOpened():
                print("[INFO] 尝试连接视频源：", VIDEO_SOURCE)
                cap = cv2.VideoCapture(VIDEO_SOURCE)

                if not cap.isOpened():
                    print("[ERROR] 无法打开视频源，2 秒后重试...")
                    if cap is not None:
                        cap.release()
                    cap = None
                    time.sleep(2)
                    continue

                print("[INFO] 视频源连接成功。")
                fail_count = 0

            # 读取一帧
            ret, frame = cap.read()
            if not ret or frame is None:
                fail_count += 1
                print(f"[WARN] 读取帧失败（{fail_count}/{MAX_FRAME_FAILS}），0.1 秒后重试...")
                time.sleep(0.1)

                # 连续多次失败：断开并重连
                if fail_count >= MAX_FRAME_FAILS:
                    print("[INFO] 连续读取帧失败次数过多，重置视频连接...")
                    cap.release()
                    cap = None
                    fail_count = 0
                continue

            # 一旦成功读到帧，失败计数清零
            fail_count = 0

            frame_idx += 1

            # 降频：只在指定帧上做检测/识别
            if DETECT_EVERY_N_FRAMES > 1 and (frame_idx % DETECT_EVERY_N_FRAMES != 0):
                if SHOW_WINDOW:
                    cv2.imshow("Face Runtime", frame)
                    if cv2.waitKey(1) & 0xFF == ord('q'):
                        break
                continue

            # 检测人脸
            faces = session.face_detection(frame)
            if not faces:
                if SHOW_WINDOW:
                    cv2.imshow("Face Runtime", frame)
                    if cv2.waitKey(1) & 0xFF == ord('q'):
                        break
                continue

            # 逐个识别
            for face in faces:
                is_match, conf, identity_id, label = recognize_face(session, frame, face)

                ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                if is_match:
                    status = "MATCH"
                    name_part = label if label else f"id={identity_id}"
                    print(f"[{ts}] MATCH {name_part} id={identity_id} conf={conf:.3f}")
                else:
                    status = "UNKNOWN"
                    print(f"[{ts}] UNKNOWN id={identity_id} conf={conf:.3f}")

                # 写入 CSV 日志
                log_to_csv(timestamp=ts, label=label, confidence=conf, status=status)

                # 画框显示（可选）
                if SHOW_WINDOW:
                    x1, y1, x2, y2 = map(int, face.location)
                    cv2.rectangle(frame, (x1, y1), (x2, y2), (0, 255, 0), 2)
                    txt = label if label else f"id={identity_id}"
                    label_txt = f"{txt} {conf:.2f}"
                    cv2.putText(
                        frame,
                        label_txt,
                        (x1, max(0, y1 - 5)),
                        cv2.FONT_HERSHEY_SIMPLEX,
                        0.5,
                        (255, 255, 255),
                        1
                    )

            if SHOW_WINDOW:
                cv2.imshow("Face Runtime", frame)
                if cv2.waitKey(1) & 0xFF == ord('q'):
                    break

    finally:
        if cap is not None:
            cap.release()
        if SHOW_WINDOW:
            cv2.destroyAllWindows()
        print("[INFO] 资源已释放，程序退出。")


if __name__ == "__main__":
    main()
