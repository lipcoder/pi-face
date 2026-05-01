package local

import (
	"fmt"
	"time"

	"gocv.io/x/gocv"
)

type Local struct{}

func (local *Local)Capture() ([]byte, error) {
	imageBytes, err := getLocalImage()
	if err != nil {
		return nil, fmt.Errorf("capture local image: %w", err)
	}
	return imageBytes, nil
}

func getLocalImage() ([]byte, error) {
	webcam, err := gocv.OpenVideoCapture(0)
	if err != nil {
		return nil, fmt.Errorf("打开摄像头失败: %w", err)
	}
	defer webcam.Close()

	if !webcam.IsOpened() {
		return nil, fmt.Errorf("摄像头未打开")
	}

	img := gocv.NewMat()
	defer img.Close()

	time.Sleep(500 * time.Millisecond)

	var ok bool
	for i := 0; i < 20; i++ {
		if webcam.Read(&img) && !img.Empty() {
			ok = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ok {
		return nil, fmt.Errorf("未能从摄像头读取到图像")
	}

	buf, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		return nil, fmt.Errorf("编码 JPG 失败: %w", err)
	}
	defer buf.Close()

	imageBytes := make([]byte, len(buf.GetBytes()))
	copy(imageBytes, buf.GetBytes())

	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("JPG 图片为空")
	}

	return imageBytes, nil

}
