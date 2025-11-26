#!/bin/bash
set -e

# 1) 启动 Go 日志看板
./web/server &

# 2) 启动海康取流 + MJPEG HTTP 服务
python app/hik_mjpeg_server.py &

# 3) 启动人脸识别主程序（前两个在后台，这个前台挂起）
python app/face_runtime.py
