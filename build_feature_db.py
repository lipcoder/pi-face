#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import json

import cv2
import inspireface as isf

# ================== 路径配置（迁移时主要改这里） ==================

# 当前脚本所在目录
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# 项目 data 目录（默认为 ../data）
DATA_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, "..", "data"))

# 已知人脸目录（注意：你说的是 ../data/konw，这里按惯例写成 know，自行按需改名）
KNOW_FACE_DIR = os.path.join(DATA_DIR, "know")

# 特征数据库目录与文件
FEATURE_DB_DIR = os.path.join(DATA_DIR, "feature_db")
FEATURE_DB_PATH = os.path.join(FEATURE_DB_DIR, "feature_hub.db")
LABEL_MAP_PATH = os.path.join(FEATURE_DB_DIR, "label_map.json")

os.makedirs(KNOW_FACE_DIR, exist_ok=True)
os.makedirs(FEATURE_DB_DIR, exist_ok=True)

# 识别阈值（与运行时脚本保持一致）
SEARCH_THRESHOLD = 0.48

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


# ================== InspireFace 初始化 ==================

def init_inspireface():
    """初始化 InspireFace 会话 + FeatureHub（仅建库，不拉摄像头）"""
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


# ================== 建库逻辑 ==================

def is_image_file(path: str) -> bool:
    ext = os.path.splitext(path)[1].lower()
    return ext in [".jpg", ".jpeg", ".png", ".bmp", ".webp"]


def build_known_faces_from_dir(session):
    """
    从 KNOW_FACE_DIR 读取图片，建立已知人脸特征并写入 FeatureHub。
    规则：文件名（不含后缀）作为 label，例如：zhangsan.jpg -> "zhangsan"
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

    # 已有的 label 集合（防止重复）
    existing_labels = set(KNOWN_LABEL_MAP.values())

    for fname in files:
        path = os.path.join(KNOW_FACE_DIR, fname)
        if not os.path.isfile(path) or not is_image_file(path):
            continue

        label = os.path.splitext(fname)[0]

        if label in existing_labels:
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

        identity = isf.FaceIdentity(feature, -1)
        ret, face_id = isf.feature_hub_face_insert(identity)
        if not ret:
            print(f"[WARN] 插入 FeatureHub 失败：{path}")
            continue

        KNOWN_LABEL_MAP[str(face_id)] = label
        existing_labels.add(label)
        inserted += 1
        print(f"[INFO] 已加入已知人脸: face_id={face_id}, label={label}, file={fname}")

    if inserted > 0:
        save_label_map()

    print(f"[INFO] 已知人脸建库完成，本次新增 {inserted} 条，总数 {len(KNOWN_LABEL_MAP)}")
    print(f"[INFO] 当前库中人脸总数（FeatureHub）: {isf.feature_hub_get_face_count()}")


def main():
    print("[INFO] 使用已知人脸目录：", KNOW_FACE_DIR)
    print("[INFO] 特征库数据库：", FEATURE_DB_PATH)

    session = init_inspireface()
    build_known_faces_from_dir(session)

    print("[INFO] 建库脚本执行完毕。")


if __name__ == "__main__":
    main()
