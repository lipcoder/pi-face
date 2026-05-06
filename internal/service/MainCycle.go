package recognition

import (
	"context"
	"errors"
	"lipcoder/face/internal/adapter/camera"
	facedb "lipcoder/face/internal/database"
	"log/slog"
	"time"
)

const (
	DefaultAttendanceInterval = 500 * time.Millisecond
	DefaultCosineThreshold    = 0.45
)

type AttendanceHandler func(ctx context.Context, name string, similarity float64) error

// StartAttendanceLoop 是主签到循环。
// 这条线常驻运行：
// camera.Capture -> InspireFace -> SearchFaceByEmbedding -> onMatched
// go里面函数是一等公民，使用这个StartAttendanceLoop函数的时候AttendanceHandler就是一个函数，要将函数传进去
func (s *FaceVerifyService) StartAttendanceLoop(
	ctx context.Context,
	cam camera.Camera,
	interval time.Duration,
	threshold float64,
	onMatched AttendanceHandler,
) error {
	if cam == nil {
		return ErrNilCamera
	}

	if interval <= 0 {
		interval = DefaultAttendanceInterval
	}

	if threshold <= 0 {
		threshold = DefaultCosineThreshold
	}

	ticker := time.NewTicker(interval) //每隔interval响一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			embedding, err := s.ExtractBestEmbeddingFromCamera(ctx, cam)
			if err != nil {
				slog.Warn("attendance extract embedding failed", "err", err)
				continue
			}

			embeddingText := EmbeddingToPGVector(embedding)

			name, similarity, err := facedb.SearchFaceByEmbedding(ctx, embeddingText, threshold)
			if err != nil {
				if !errors.Is(err, facedb.ErrNotFound) {
					slog.Warn("attendance search face failed", "err", err)
				}
				continue
			}

			if onMatched != nil {
				if err := onMatched(ctx, name, similarity); err != nil {
					slog.Warn("attendance handler failed", "name", name, "similarity", similarity, "err", err)
				}
				continue
			}

			slog.Info("attendance matched", "name", name, "similarity", similarity)
		}
	}
}

// ExtractBestEmbeddingFromCamera 给主签到用。
// 如果图片里有多张脸，选质量最高的一张。
func (s *FaceVerifyService) ExtractBestEmbeddingFromCamera(
	ctx context.Context,
	cam camera.Camera,
) ([]float64, error) {
	return s.extractEmbeddingFromCamera(ctx, cam, false)
}
