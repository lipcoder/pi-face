import os
import cv2
import time
import threading
import logging
from logging.handlers import RotatingFileHandler

from flask import Flask, Response, jsonify

# 只从 config 读取“路径类配置”（例如日志文件路径）
from app.config import (
    HIK_IP, HIK_USER, HIK_PWD, HIK_PORT,
    HIK_CHANNEL_MAIN, HIK_CHANNEL_SUB,
    HTTP_HOST, HTTP_PORT,
    LOG_PATH,
)

OPEN_TIMEOUT_SEC = 6          # 单个 URL 探测超时
TEST_FRAMES = 8               # 探测阶段需要成功读取的帧数
RETRY_ALL_URL_SLEEP_SEC = 1.0 # 所有 URL 都不可用后，多久重试一轮
READ_FAIL_SLEEP_SEC = 0.2     # 单次读帧失败后的小睡眠
MAX_CONSECUTIVE_FAILS = 30    # 连续读帧失败多少次后，直接释放并重走“全流程”

# OpenCV/FFmpeg 的 RTSP 可靠性常常靠“强制 TCP”救命；这里作为文件内策略写死。
# 如需覆盖，请在 Docker 里设置环境变量 OPENCV_FFMPEG_CAPTURE_OPTIONS（OpenCV 会读取）。
os.environ.setdefault(
    "OPENCV_FFMPEG_CAPTURE_OPTIONS",
    "rtsp_transport;tcp|stimeout;5000000|max_delay;500000"
)


def _setup_logger() -> logging.Logger:
    logger = logging.getLogger("hik_mjpeg")
    logger.setLevel(logging.INFO)
    logger.propagate = False

    # 防止重复添加 handler（例如热重启 / 多次 import）
    if logger.handlers:
        return logger

    log_dir = os.path.dirname(LOG_PATH)
    if log_dir:
        os.makedirs(log_dir, exist_ok=True)

    file_handler = RotatingFileHandler(
        LOG_PATH, maxBytes=10 * 1024 * 1024, backupCount=3, encoding="utf-8"
    )
    file_handler.setFormatter(logging.Formatter(
        fmt="%(asctime)s [%(levelname)s] %(threadName)s %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    ))

    console_handler = logging.StreamHandler()
    console_handler.setFormatter(logging.Formatter(
        fmt="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%H:%M:%S",
    ))

    logger.addHandler(file_handler)
    logger.addHandler(console_handler)
    return logger


logger = _setup_logger()

app = Flask(__name__)

frame_lock = threading.Lock()
latest_jpeg: bytes | None = None
stop_flag = False


def _rtsp_urls() -> list[str]:
    base = f"rtsp://{HIK_USER}:{HIK_PWD}@{HIK_IP}:{HIK_PORT}"
    return [
        f"{base}/{HIK_CHANNEL_MAIN}?transportmode=unicast",
        f"{base}/{HIK_CHANNEL_SUB}?transportmode=unicast",
    ]


def _try_open_stream(url: str) -> cv2.VideoCapture | None:
    """尝试打开并在限定时间内读取若干帧；成功才算可用。"""
    start = time.time()
    cap = cv2.VideoCapture(url, cv2.CAP_FFMPEG)
    if not cap.isOpened():
        cap.release()
        return None

    ok_count = 0
    while time.time() - start < OPEN_TIMEOUT_SEC:
        ok, frame = cap.read()
        if ok and frame is not None and frame.size > 0:
            ok_count += 1
            if ok_count >= TEST_FRAMES:
                return cap
        else:
            time.sleep(0.05)

    cap.release()
    return None


def capture_loop():
    global latest_jpeg, stop_flag

    urls = _rtsp_urls()
    working_cap: cv2.VideoCapture | None = None
    working_url: str | None = None
    consecutive_fails = 0

    logger.info("候选 RTSP URLs: %s", urls)

    while not stop_flag:
        # 1) 如果没有可用 cap：完整走一遍“选择可用 URL → 打开会话”的流程
        if working_cap is None:
            working_url = None
            consecutive_fails = 0

            for url in urls:
                if stop_flag:
                    break
                logger.info("尝试打开: %s", url)
                cap = _try_open_stream(url)
                if cap is not None:
                    working_cap = cap
                    working_url = url
                    logger.info("已连接: %s", url)
                    break
                else:
                    logger.warning("打开失败: %s", url)

            if working_cap is None:
                logger.error("所有候选流均不可用，%ss 后重试完整流程", RETRY_ALL_URL_SLEEP_SEC)
                time.sleep(RETRY_ALL_URL_SLEEP_SEC)
                continue

        # 2) 有 cap：持续拉帧；出现持续失败则 release 并回到步骤 1
        ok, frame = working_cap.read()
        if not ok or frame is None:
            consecutive_fails += 1
            logger.warning(
                "读取帧失败 (%d/%d) url=%s",
                consecutive_fails, MAX_CONSECUTIVE_FAILS, working_url
            )
            if consecutive_fails >= MAX_CONSECUTIVE_FAILS:
                logger.error(
                    "连续读帧失败达到阈值，释放会话并重走完整流程 url=%s",
                    working_url
                )
                try:
                    working_cap.release()
                except Exception:
                    logger.exception("release() 过程中异常")
                working_cap = None
                working_url = None
                consecutive_fails = 0
            time.sleep(READ_FAIL_SLEEP_SEC)
            continue

        consecutive_fails = 0

        # 编码为 JPEG
        ok2, buf = cv2.imencode(".jpg", frame)
        if not ok2:
            logger.warning("JPEG 编码失败，丢弃该帧 url=%s", working_url)
            continue

        with frame_lock:
            latest_jpeg = buf.tobytes()


@app.get("/health")
def health():
    with frame_lock:
        has_frame = latest_jpeg is not None
    return jsonify({"ok": True, "has_frame": has_frame})


@app.get("/snapshot")
def snapshot():
    with frame_lock:
        data = latest_jpeg
    if not data:
        return Response("no frame", status=503, mimetype="text/plain")
    return Response(data, mimetype="image/jpeg")


@app.get("/video_feed")
def video_feed():
    def gen():
        boundary = b"--frame"
        while not stop_flag:
            with frame_lock:
                data = latest_jpeg
            if not data:
                time.sleep(0.05)
                continue

            yield boundary + b"\r\n"
            yield b"Content-Type: image/jpeg\r\n"
            yield b"Content-Length: " + str(len(data)).encode("ascii") + b"\r\n\r\n"
            yield data + b"\r\n"
            time.sleep(0.03)

    return Response(gen(), mimetype="multipart/x-mixed-replace; boundary=frame")


def main():
    global stop_flag

    t = threading.Thread(target=capture_loop, name="capture", daemon=True)
    t.start()

    logger.info("启动 HTTP MJPEG 服务: http://%s:%s/", HTTP_HOST, HTTP_PORT)
    logger.info("MJPEG 流地址: http://%s:%s/video_feed", HTTP_HOST, HTTP_PORT)
    logger.info("单帧调试地址: http://%s:%s/snapshot", HTTP_HOST, HTTP_PORT)
    logger.info("日志文件: %s", LOG_PATH)

    try:
        app.run(host=HTTP_HOST, port=HTTP_PORT, debug=False, threaded=True)
    finally:
        stop_flag = True
        t.join(timeout=2)
        logger.info("程序退出")


if __name__ == "__main__":
    main()
