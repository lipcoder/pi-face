# 不知道为什么会有这个占用栈的问题，只能这样清理一下再用

python - <<'PY'
import glob, subprocess, sys
paths = glob.glob(sys.prefix + "/lib/python*/site-packages/inspireface/modules/core/libs/linux/*/libInspireFace.so")
for so in paths:
    subprocess.check_call(["patchelf", "--clear-execstack", so])
    print("patched:", so)
PY
