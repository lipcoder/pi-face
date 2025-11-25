import cv2
import time
import threading

from flask import Flask, Response, render_template_string

# ======== 配置区域 ========
IP = "192.168.1.111"
USER = "admin"
PWD = "Long-Live-NBALab"
PORT = 554              # RTSP 端口，一般海康默认 554
HTTP_HOST = "0.0.0.0"   # 对外监听地址，0.0.0.0 表示所有网卡
HTTP_PORT = 5000        # 本地 HTTP 端口
# =======================

# 海康常见 RTSP URL 候选列表
CANDIDATE_URLS = [
    # 新格式，主码流 / 子码流
    f"rtsp://{USER}:{PWD}@{IP}:{PORT}/Streaming/Channels/101",
    f"rtsp://{USER}:{PWD}@{IP}:{PORT}/Streaming/Channels/101?transportmode=unicast",
    f"rtsp://{USER}:{PWD}@{IP}:{PORT}/Streaming/Channels/102",
    f"rtsp://{USER}:{PWD}@{IP}:{PORT}/Streaming/Channels/102?transportmode=unicast",

]

latest_frame = None
frame_lock = threading.Lock()
stop_flag = False

app = Flask(__name__)


def try_open_stream(url, test_frames=30, timeout_sec=5):
    """尝试打开一个 RTSP URL，读取几帧看是否真的有画面"""
    print(f"\n[INFO] 尝试连接: {url}")
    cap = cv2.VideoCapture(url)

    if not cap.isOpened():
        print("[WARN] VideoCapture 打不开这个 URL")
        cap.release()
        return None

    start_time = time.time()
    frame_count = 0

    while frame_count < test_frames and (time.time() - start_time) < timeout_sec:
        ret, frame = cap.read()
        if not ret or frame is None:
            continue
        frame_count += 1

    if frame_count == 0:
        print("[WARN] 虽然连接上了，但没有读到有效帧")
        cap.release()
        return None

    print(f"[OK] 成功从 {url} 读到 {frame_count} 帧")
    return cap


def capture_thread_func():
    """后台线程：持续从摄像头读取帧，更新 latest_frame"""
    global latest_frame, stop_flag

    working_cap = None

    # 依次尝试候选 RTSP URL，找到第一个能用的
    for url in CANDIDATE_URLS:
        cap = try_open_stream(url)
        if cap is not None:
            working_cap = cap
            print(f"\n[INFO] 使用工作中的 RTSP URL: {url}")
            break

    if working_cap is None:
        print("\n[ERROR] 所有候选 RTSP URL 都无法成功取流，退出采集线程。")
        stop_flag = True
        return

    while not stop_flag:
        ret, frame = working_cap.read()
        if not ret or frame is None:
            print("[WARN] 读取帧失败，等待 0.5 秒后重试...")
            time.sleep(0.5)
            continue

        with frame_lock:
            latest_frame = frame

    working_cap.release()
    print("[INFO] 采集线程已退出")


def mjpeg_generator():
    """Flask 使用的生成器，将 latest_frame 编码为 JPEG，并以 MJPEG 形式输出"""
    global latest_frame, stop_flag

    print("[INFO] 新的 MJPEG 客户端连接")
    while not stop_flag:
        with frame_lock:
            frame = None if latest_frame is None else latest_frame.copy()

        if frame is None:
            # 还没拿到任何帧，稍微等一下
            time.sleep(0.05)
            continue

        # 编码为 JPEG
        ret, jpeg = cv2.imencode(".jpg", frame)
        if not ret:
            print("[WARN] JPEG 编码失败")
            continue

        jpg_bytes = jpeg.tobytes()

        # multipart/x-mixed-replace 的一帧
        yield (
            b"--frame\r\n"
            b"Content-Type: image/jpeg\r\n\r\n" +
            jpg_bytes +
            b"\r\n"
        )

    print("[INFO] MJPEG 生成器结束")


# ===== Flask 路由 =====

HTML_PAGE = """
<!doctype html>
<html>
  <head>
    <title>Hikvision Preview</title>
    <meta charset="utf-8" />
    <style>
      body { font-family: sans-serif; }
      img { max-width: 100%; }
    </style>
  </head>
  <body>
    <h1>Hikvision Preview (auto refresh)</h1>
    <p>每 200ms 刷新一帧，看到的就是程序当前捕获的画面。</p>
    <img id="cam" src="/snapshot" />
    <script>
      function reloadImage() {
        var img = document.getElementById("cam");
        // 加时间戳防止浏览器缓存
        img.src = "/snapshot?t=" + new Date().getTime();
      }
      // 200ms 刷新一次
      setInterval(reloadImage, 200);
    </script>
  </body>
</html>
"""



@app.route("/")
def index():
    return render_template_string(HTML_PAGE)


@app.route("/video_feed")
def video_feed():
    """MJPEG 视频流接口"""
    return Response(
        mjpeg_generator(),
        mimetype="multipart/x-mixed-replace; boundary=frame"
    )


@app.route("/snapshot")
def snapshot():
    """返回当前最新的一帧 JPEG，便于调试"""
    global latest_frame
    with frame_lock:
        frame = None if latest_frame is None else latest_frame.copy()

    if frame is None:
        return "no frame yet", 503

    ret, jpeg = cv2.imencode(".jpg", frame)
    if not ret:
        return "encode failed", 500

    return Response(jpeg.tobytes(), mimetype="image/jpeg")


def main():
    global stop_flag

    # 启动采集线程
    t = threading.Thread(target=capture_thread_func, daemon=True)
    t.start()

    print(f"[INFO] 启动 HTTP MJPEG 服务: http://{HTTP_HOST}:{HTTP_PORT}/")
    print(f"[INFO] MJPEG 流地址: http://{HTTP_HOST}:{HTTP_PORT}/video_feed")
    print(f"[INFO] 单帧调试地址: http://{HTTP_HOST}:{HTTP_PORT}/snapshot")

    try:
        app.run(host=HTTP_HOST, port=HTTP_PORT, debug=False, threaded=True)
    finally:
        stop_flag = True
        t.join(timeout=2)
        print("[INFO] 程序退出")


if __name__ == "__main__":
    main()
