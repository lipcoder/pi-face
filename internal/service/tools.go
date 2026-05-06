package recognition

import (
	"context"
	"errors"
	"fmt"
	"lipcoder/face/internal/adapter/camera"
	"strconv"
	"strings"

	facejson "lipcoder/face/internal/json"
)

// 暂定，下面会共享给用户逻辑里面
func (s *FaceVerifyService) extractEmbeddingFromCamera(
	ctx context.Context,
	cam camera.Camera,
	requireSingleFace bool,
) ([]float64, error) {
	if cam == nil {
		return nil, ErrNilCamera
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	imageBytes, err := cam.Capture()
	if err != nil {
		return nil, fmt.Errorf("capture image: %w", err)
	}

	return s.extractEmbeddingFromImage(ctx, imageBytes, requireSingleFace)
}

func (s *FaceVerifyService) extractEmbeddingFromImage(
	ctx context.Context,
	imageBytes []byte,
	requireSingleFace bool,
) ([]float64, error) {
	if len(imageBytes) == 0 {
		return nil, ErrImageEmpty
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	respBody, err := s.inspire.PostImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("post image to inspireface: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	embedding, err := getBestEmbeddingFromInspireResponse(respBody, requireSingleFace)
	if err != nil {
		return nil, fmt.Errorf("get embedding from inspireface response: %w", err)
	}

	return embedding, nil
}

func getBestEmbeddingFromInspireResponse(
	respBody []byte,
	requireSingleFace bool,
) ([]float64, error) {
	response, err := facejson.BytesToResponse(respBody)
	if err != nil {
		return nil, err
	}

	if !response.OK {
		return nil, errors.New("inspireface response ok is false")
	}

	if len(response.Faces) == 0 {
		return nil, ErrNoFaceDetected
	}

	if requireSingleFace && len(response.Faces) != 1 {
		return nil, ErrMultipleFaces
	}

	bestFace := response.Faces[0]

	for _, face := range response.Faces[1:] {
		if face.Quality > bestFace.Quality {
			bestFace = face
		}
	}

	if len(bestFace.Embedding) == 0 {
		return nil, ErrEmbeddingEmpty
	}

	return bestFace.Embedding, nil
}

// 把 []float64 转成 pgvector 能接收的字符串
func EmbeddingToPGVector(embedding []float64) string {
	var builder strings.Builder

	builder.WriteByte('[')

	for i, value := range embedding {
		if i > 0 {
			builder.WriteByte(',')
		}

		builder.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	}

	builder.WriteByte(']')

	return builder.String()
}
