package service

import (
	"context"
	"errors"
	"fmt"
	"lipcoder/face/internal/camera"
	"lipcoder/face/internal/data"
	"lipcoder/face/internal/recognition"
	"time"
)

const (
	DefaultFaceInterval   = 500 * time.Millisecond
	DefaultFaceSimilarity = 0.45
	DefaultFaceQuality    = 0.45
)

// 每隔interval获取一次图像
func SignIn(
	ctx context.Context,
	cam camera.Camera,
	rec recognition.Recognition,
	facedb data.Facedb,
	interval time.Duration,
	similarity float64,
) error {
	if cam == nil {
		return fmt.Errorf("camera cannot be nil")
	}
	if rec == nil {
		return fmt.Errorf("recognition cannot be nil")
	}

	if interval <= 0 {
		interval = DefaultFaceInterval
	}

	if similarity <= 0 {
		similarity = DefaultFaceSimilarity
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			bestembedding, err := ExtractBestEmbeddingFromCamera(ctx, cam, rec)
			if err != nil {
				return fmt.Errorf("attendance extract embedding failed %w", err)
			} else if bestembedding == nil {
				continue
			}

			name, facesimilarity, err := facedb.SearchFaceByEmbedding(ctx, bestembedding, similarity)
			if err != nil {
				if !errors.Is(err, data.ErrNotFound) {
					continue
				} else {
					return fmt.Errorf("attendance search face failed %w", err)
				}
			}

			err = RecordFaceSimilarity(name, facesimilarity)
			if err != nil {
				return fmt.Errorf("write attendance record file %w", err)
			}
		}
	}
}

func ExtractBestEmbeddingFromCamera(
	ctx context.Context,
	cam camera.Camera,
	rec recognition.Recognition,
) ([]float64, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	imageBytes, err := cam.Capture()
	if err != nil {
		return nil, fmt.Errorf("mainCycle get image failed,%w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	embedding, err := rec.GetFaceEmbedding(imageBytes, 1)
	if err != nil {
		return nil, fmt.Errorf("get embedding from inspireface response: %w", err)
	} else if embedding == nil {
		return nil, nil
	}else if len(embedding)==0 {
		return nil,nil
	}

	return embedding[0], nil
}
