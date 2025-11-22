#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import time
import json
from datetime import datetime

import cv2
import numpy as np
import inspireface as isf

# ================== 配置区域 ==================

# 摄像头 / 视频流来源：改为本地 5000 端口
VIDEO_SOURCE = os.environ.get("VIDEO_SOURCE", "http://127.0.0.1:5000/video_feed")

# 已知人脸目录
KNOW_FACE_DIR = "/data/know"
os.makedirs(KNOW_FACE_DIR, exist_ok=True)

# 未知（抓拍）人脸保存目录
FACE_SAVE_DIR = "/data/unknow"
os.makedirs(FACE_SAVE_DIR, exist_ok=True)

# 特征数据库
FEATURE_DB_DIR = "/data/feature_db"
FEATURE_DB_PATH = os.path.join(FEATURE_DB_DIR, "feature_hub.db")
LABEL_MAP_PATH = os.path.join(FEATURE_DB_DIR, "label_map.json")
os.makedirs(FEATURE_DB_DIR, exist_ok=True)

# 识别阈值
SEARCH_THRESHOLD = 0.48

# 是否显示调试窗口
SHOW_WINDOW = False

# 全局：face_id -> label 的映射
KNOWN_LABEL_MAP = {}


# ================== 工具函数：label map ==================

def load_label_map():
    global KNOWN_LABEL_MAP
    if os.path.exists(LABEL_MAP_PATH):
        try:
            with open(LABEL_MAP_PATH, "r", encoding="utf-8") as f:
                KNOWN_LABEL_MAP = json.load(f)
            print(f"[INFO] 已加载 label_map: {len(KNOWN_LABEL_MAP)} 条")
        except Exception as e:
            print("[WARN] 读取 label_map 失败，将重新构建：", e)
            KNOWN_LABEL_MAP = {}
    else:
        KNOWN_LABEL_MAP = {}


def save_label_map():
    try:
        with open(LABEL_MAP_PATH, "w", encoding="utf-8") as f:
            json.dump(KNOWN_LABEL_MAP, f, ensure_ascii=False, indent=2)
        print(f"[INFO] 已保存 label_map: {len(KNOWN_LABEL_MAP)} 条 -> {LABEL_MAP_PATH}")
    except Exception as e:
        print("[WARN] 保存 label_map 失败：", e)


# ================== 初始化 InspireFace ==================

def init_inspireface():
    """
    初始化 InspireFace 会话 + FeatureHub + 预载 /data/know 已知人脸
    """
    try:
        isf.reload("Pikachu")
    except Exception as e:
        print("[WARN] reload Pikachu 失败，可能你已经在 C++ 侧配置好模型了：", e)

    opt = isf.HF_ENABLE_FACE_RECOGNITION

    # 注意：使用位置参数，不要用 opt=opt
    session = isf.InspireFaceSession(
        opt,
        isf.HF_DETECT_MODE_ALWAYS_DETECT,
    )

    session.set_detection_confidence_threshold(0.5)

    # 启用 FeatureHub（sqlite）
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

    # 先加载已有 label_map
    load_label_map()

    # 如果没有 label_map，则从 /data/know 生成一遍
    if not KNOWN_LABEL_MAP:
        build_known_faces_from_dir(session)

    return session


# ================== 已知人脸建库 ==================

def is_image_file(path: str) -> bool:
    ext = os.path.splitext(path)[1].lower()
    return ext in [".jpg", ".jpeg", ".png", ".bmp", ".webp"]


def build_known_faces_from_dir(session):
    """
    从 /data/know 目录读取图片，建立已知人脸特征并写入 FeatureHub。
    规则：文件名（不含后缀）作为 label，例如：/data/know/zhangsan.jpg -> "zhangsan"
    """
    global KNOWN_LABEL_MAP

    if not os.path.isdir(KNOW_FACE_DIR):
        print(f"[WARN] 已知人脸目录不存在：{KNOW_FACE_DIR}")
        return

    files = sorted(os.listdir(KNOW_FACE_DIR))
    if not files:
        print(f"[WARN] 已知人脸目录为空：{KNOW_FACE_DIR}")
        return

    inserted = 0

    for fname in files:
        path = os.path.join(KNOW_FACE_DIR, fname)
        if not os.path.isfile(path) or not is_image_file(path):
            continue

        label = os.path.splitext(fname)[0]

        # 已经有同名 label 的就跳过（防止重复插入）
        if label in KNOWN_LABEL_MAP.values():
            print(f"[INFO] 跳过已存在 label: {label}")
            continue

        img = cv2.imread(path)
        if img is None:
            print(f"[WARN] 无法读取图片：{path}")
            continue

        faces = session.face_detection(img)
        if not faces:
            print(f"[WARN] 未检测到人脸：{path}")
            continue

        face = faces[0]

        feature = session.face_feature_extract(img, face)
        if feature is None or feature.size == 0:
            print(f"[WARN] 未能提取特征：{path}")
            continue

        # 构造 FaceIdentity，这里第二个参数给个占位 id（在 AUTO_INCREMENT 模式下实际主键由 DB 分配）
        identity = isf.FaceIdentity(feature, -1)

        ret, face_id = isf.feature_hub_face_insert(identity)
        if not ret:
            print(f"[WARN] 插入 FeatureHub 失败：{path}")
            continue

        KNOWN_LABEL_MAP[str(face_id)] = label
        inserted += 1
        print(f"[INFO] 已加入已知人脸: face_id={face_id}, label={label}, file={fname}")

    if inserted > 0:
        save_label_map()

    print(f"[INFO] 已知人脸建库完成，本次新增 {inserted} 条，总数 {len(KNOWN_LABEL_MAP)}")


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
    把截取的人脸图保存到 /data/unknow 下
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

    # 正确的用法：SearchResult 对象
    # confidence 在 result.confidence
    # id 在 result.similar_identity.id
    confidence = getattr(result, "confidence", 0.0)

    similar_identity = getattr(result, "similar_identity", None)
    if similar_identity is not None:
        identity_id = getattr(similar_identity, "id", -1)
    else:
        identity_id = -1

    is_match = confidence >= SEARCH_THRESHOLD

    # 从 label_map.json 里取名字
    label = KNOWN_LABEL_MAP.get(str(identity_id))

    return is_match, float(confidence), int(identity_id), label




# ================== 主循环：从 5000 端口拉流 + 截脸再识别 ==================

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

            # 1. 人脸检测（在整帧上做）
            faces = session.face_detection(frame)
            if not faces:
                if SHOW_WINDOW:
                    cv2.imshow("Face Runtime", frame)
                    if cv2.waitKey(1) & 0xFF == ord('q'):
                        break
                continue

            for face in faces:
                # 2. 截取人脸子图
                face_img = crop_face_from_frame(frame, face)
                if face_img is None:
                    continue

                face_path = save_face_image(face_img)

                # 3. 识别
                is_match, conf, identity_id, label = recognize_face(session, frame, face)

                ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                if is_match:
                    name_part = label if label else f"id={identity_id}"
                    print(f"[{ts}] MATCH {name_part} id={identity_id} conf={conf:.3f} img={face_path}")
                else:
                    print(f"[{ts}] UNKNOWN id={identity_id} conf={conf:.3f} img={face_path}")

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
