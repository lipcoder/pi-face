package inspireface

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"lipcoder/face/internal/config"
)

type Inspire struct {
	client *http.Client
}

var (
	ErrBuildImageRequest = errors.New("build image request failed")
	ErrPostImageRequest  = errors.New("post image request failed")
	ErrPostImageResponse = errors.New("post image response failed")
)

func NewInspire(client *http.Client) *Inspire {
	if client == nil {
		client = http.DefaultClient
	}

	return &Inspire{
		client: client,
	}
}

func (ins *Inspire) PostImage(imgBytes []byte) ([]byte, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("%w: create form file failed: %w", ErrBuildImageRequest, err)
	}

	if _, err := part.Write(imgBytes); err != nil {
		return nil, fmt.Errorf("%w: write image bytes failed: %w", ErrBuildImageRequest, err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("%w: close multipart writer failed: %w", ErrBuildImageRequest, err)
	}

	con := config.Load()

	req, err := http.NewRequest(http.MethodPost, con.InspireFaceHost, &body)
	if err != nil {
		return nil, fmt.Errorf("%w: create request failed: %w", ErrBuildImageRequest, err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := ins.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: do request failed: %w", ErrPostImageRequest, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response failed: %w", ErrPostImageResponse, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"%w: status=%d, body=%s",
			ErrPostImageResponse,
			resp.StatusCode,
			string(respBody),
		)
	}

	return respBody, nil
}
