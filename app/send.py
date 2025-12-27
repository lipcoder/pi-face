import sys, time, serial

PORT="/dev/ttyACM0"
BAUD=115200

s = serial.Serial(PORT, BAUD, timeout=0.2)
# 第一次打开会复位，等它启动一次
time.sleep(2.0)

print("Ready. Type ok/no/off, Ctrl-D to quit.")
for line in sys.stdin:
    cmd = line.strip()
    if not cmd:
        continue
    s.write((cmd + "\n").encode())
    s.flush()
