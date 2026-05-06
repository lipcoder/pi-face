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
	return &Hik{
		client: client,
	}
}

var (
	ErrUrl     = errors.New("url failed")
	ErrRequest = errors.New("request failed")
	ErrImage   = errors.New("image failed")
)

func (a *Hik) Capture() ([]byte, error) {
	con := config.Load()

	imageBytes, err := a.getWebImage(con.HikvisionHost, con.HikvisionUsername, con.HikvisionPassword)
	if err != nil {
		return nil, fmt.Errorf("capture hikvision image: %w", err)
	}

	return imageBytes, nil
}

func (a *Hik) getWebImage(URL, username, passwd string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w create request failed: %w", ErrUrl, err)
	}

	req.SetBasicAuth(username, passwd)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w send request failed: %w", ErrRequest, err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

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
