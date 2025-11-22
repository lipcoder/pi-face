
docker run --rm -it \
  --name=face \
  --mount type=bind,source="$HOME/project/face/data",target=/data \
  --network host \
  face:v3 \
  /bin/bash
