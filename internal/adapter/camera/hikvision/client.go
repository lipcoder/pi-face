package hikvision

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"lipcoder/face/internal/config"
)

type Hik struct {
	client *http.Client
}

func NewHik(client *http.Client) *Hik {
	if client == nil {
		client = http.DefaultClient
	}

	return &Hik{
		client: client,
	}
}

func (hik *Hik) Capture() ([]byte, error) {
	con := config.Load()

	imageBytes, err := hik.getWebImage(con.HikvisionHost, con.HikvisionUsername, con.HikvisionPassword)
	if err != nil {
		return nil, fmt.Errorf("capture hikvision image: %w", err)
	}

	return imageBytes, nil
}

var (
	ErrImage   = errors.New("image")
	ErrRequest = errors.New("request")
)

func (hik *Hik) getWebImage(URL, username, passwd string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	// *url.Error，URL格式错误
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.SetBasicAuth(username, passwd)
	// *url.Error，如果同时为请求超时则有两条日志
	resp, err := hik.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	// ErrRequest
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"%w: status=%d, contentType=%s, body=%s",
			ErrRequest,
			resp.StatusCode,
			contentType,
			string(body),
		)
	}

	// ErrImage
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body failed: %w", ErrImage, err)
	}

	// ErrImage
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("%w: image is empty", ErrImage)
	}

	return imageBytes, nil
}
