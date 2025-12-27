#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import json
import logging
import hashlib

import cv2
import inspireface as isf

from app.config import (
    DATA_ROOT,
    KNOW_DIR,
    FEATURE_DB_DIR,
    FEATURE_DB_PATH,
    LABEL_MAP_PATH,
    LOG_DIR,
    SEARCH_THRESHOLD,
)


# 日志系统（与 face_runtime 共用 1.txt）
os.makedirs(LOG_DIR, exist_ok=True)
LOG_FILE = os.path.join(LOG_DIR, "1.txt")

logger = logging.getLogger("build_feature_db")
logger.setLevel(logging.INFO)
logger.propagate = False

if not logger.handlers:
    handler = logging.FileHandler(LOG_FILE, encoding="utf-8")
    handler.setFormatter(logging.Formatter(
        fmt="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    ))
    logger.addHandler(handler)


# 工具函数
def is_image_file(fname: str) -> bool:
    ext = os.path.splitext(fname)[1].lower()
    return ext in (".jpg", ".jpeg", ".png", ".bmp", ".webp")



# label_map
def load_label_map():
    if os.path.exists(LABEL_MAP_PATH):
        with open(LABEL_MAP_PATH, "r", encoding="utf-8") as f:
            data = json.load(f)
        logger.info("label_map loaded: %d entries", len(data))
        return data
    logger.info("label_map not found, start from empty")
    return {}

def save_label_map(label_map: dict):
    with open(LABEL_MAP_PATH, "w", encoding="utf-8") as f:
        json.dump(label_map, f, ensure_ascii=False, indent=2)
    logger.info("label_map saved: %d entries", len(label_map))


# InspireFace
def init_inspireface():
    try:
        isf.reload("Pikachu")
    except Exception:
        logger.exception("inspireface reload failed")

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

    if not isf.feature_hub_enable(cfg):
        raise RuntimeError("FeatureHub enable failed")

    logger.info(
        "FeatureHub enabled, face_count=%d",
        isf.feature_hub_get_face_count()
    )
    return session



def build_from_know_dir(session):
    label_map = load_label_map()
    existing_labels = set(label_map.values())

    if not os.path.isdir(KNOW_DIR):
        logger.error("know dir not exists: %s", KNOW_DIR)
        return

    files = sorted(os.listdir(KNOW_DIR))
    if not files:
        logger.warning("know dir is empty: %s", KNOW_DIR)
        return

    inserted = 0
    skipped = 0

    for fname in files:
        if not is_image_file(fname):
            continue

        label = os.path.splitext(fname)[0]
        path = os.path.join(KNOW_DIR, fname)

        if label in existing_labels:
            logger.info("SKIP label exists: %s (%s)", label, fname)
            skipped += 1
            continue

        img = cv2.imread(path)
        if img is None:
            logger.warning("read image failed: %s", path)
            continue

        faces = session.face_detection(img)
        if not faces:
            logger.warning("no face detected: %s", path)
            continue

        face = faces[0]
        feature = session.face_feature_extract(img, face)
        if feature is None or feature.size == 0:
            logger.warning("feature extract failed: %s", path)
            continue

        identity = isf.FaceIdentity(feature, -1)
        ok, face_id = isf.feature_hub_face_insert(identity)
        if not ok:
            logger.error("feature_hub insert failed: %s", path)
            continue

        label_map[str(face_id)] = label
        existing_labels.add(label)
        inserted += 1

        logger.info(
            "NEW FACE inserted: face_id=%s label=%s file=%s",
            face_id, label, fname
        )

    save_label_map(label_map)

    logger.info(
        "BUILD FINISHED: inserted=%d skipped=%d total_labels=%d",
        inserted, skipped, len(label_map)
    )



def main():
    logger.info("=" * 60)
    logger.info("build_feature_db START")
    logger.info("data_root=%s", DATA_ROOT)
    logger.info("know_dir=%s", KNOW_DIR)
    logger.info("=" * 60)

    session = init_inspireface()
    build_from_know_dir(session)

    logger.info("build_feature_db FINISH")
    logger.info("=" * 60)


if __name__ == "__main__":
    main()
