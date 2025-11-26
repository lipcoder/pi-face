#!/usr/bin/env bash
set -euo pipefail

echo "[ENTRYPOINT] starting pi-face container..."

# 容器内默认使用 /data（可以通过 -e DATA_ROOT=/xxx 覆盖）
export DATA_ROOT="${DATA_ROOT:-/data}"

# 确保数据目录存在（外面挂卷进来没关系，mkdir -p 是幂等的）
mkdir -p "${DATA_ROOT}/know" \
  "${DATA_ROOT}/unknow" \
  "${DATA_ROOT}/feature_db" \
  "${DATA_ROOT}/logs"

# 如果用户传了自定义命令（比如只想跑建库脚本），就直接执行这个命令
# 例： docker run --rm pi-face python -m app.build_feature_db
if [ "$#" -gt 0 ]; then
  echo "[ENTRYPOINT] custom command: $*"
  exec "$@"
fi

# 默认行为：起两个服务

echo "[ENTRYPOINT] starting hik_mjpeg_server (RTSP -> MJPEG)..."
python -m app.hik_mjpeg_server &
HIK_PID=$!

# 给摄像头连接 & Flask 启动一点时间（防止 face_runtime 启动太快一直连不上）
sleep 3

echo "[ENTRYPOINT] starting face_runtime (face recognition loop)..."
python -m app.face_runtime &
FACE_PID=$!

term_handler() {
  echo "[ENTRYPOINT] caught termination signal, stopping child processes..."
  kill "${HIK_PID}" "${FACE_PID}" 2>/dev/null || true
  wait "${HIK_PID}" "${FACE_PID}" 2>/dev/null || true
  exit 0
}

trap term_handler SIGTERM SIGINT

# 任何一个子进程挂掉，就把另一个也杀掉，保证容器退出状态可见
set +e
wait -n "${HIK_PID}" "${FACE_PID}"
EXIT_CODE=$?
echo "[ENTRYPOINT] one of the services exited with code ${EXIT_CODE}, stopping the other..."
kill "${HIK_PID}" "${FACE_PID}" 2>/dev/null || true
wait "${HIK_PID}" "${FACE_PID}" 2>/dev/null || true
exit "${EXIT_CODE}"
