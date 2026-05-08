package local

import (
	"errors"
	"fmt"
	"time"

	"gocv.io/x/gocv"
)

type Local int

var (
	ErrNilCamera = errors.New("camera failed")
	ErrNilImages = errors.New("images failed")
)

func NewLocalCamera(a int) (*Local, error)

func (a Local) Capture() ([]byte, error) {
	imageBytes, err := a.getLocalImage()
	if err != nil {
		return nil, fmt.Errorf("capture local image: %w", err)
	}

	return imageBytes, nil
}

// 获取本地摄像头的照片
func (a Local) getLocalImage() ([]byte, error) {
	webcam, err := gocv.OpenVideoCapture(a)
	if err != nil {
		return nil, fmt.Errorf("%w Open local camera failed %w", ErrNilCamera, err)
	}
	defer webcam.Close()

	if !webcam.IsOpened() {
		return nil, ErrNilCamera
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
		return nil, ErrNilCamera
	}

	buf, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		return nil, fmt.Errorf("%w Encoding JPG failed %w", ErrNilImages, err)
	}
	defer buf.Close()

	imageBytes := make([]byte, len(buf.GetBytes()))
	copy(imageBytes, buf.GetBytes())

	if len(imageBytes) == 0 {
		return nil, ErrNilImages
	}

	return imageBytes, nil
}
