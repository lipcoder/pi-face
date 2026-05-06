package recognition

import (
	"context"
	"errors"
	"fmt"
	"lipcoder/face/internal/adapter/camera"
	"lipcoder/face/internal/adapter/inspireface"
	facedb "lipcoder/face/internal/database"
	"net/http"
	"strings"
)

const DefaultMaxAddFaceJobs = 2

type FaceVerifyService struct {
	inspire *inspireface.Inspire
	// 控制 add 人脸任务的并发数,主签到不走这个限制，避免录入任务影响签到主流程
	addFaceSem chan struct{}
}

func NewFaceVerifyService(httpClient *http.Client) *FaceVerifyService {
	return &FaceVerifyService{
		inspire:    inspireface.NewInspire(httpClient),
		addFaceSem: make(chan struct{}, DefaultMaxAddFaceJobs),
	}
}

// AddFaceFromCamera 是添加人脸的核心流程。
// 用户只传 name 和图片来源。
// 这里要求图片里只能有一张脸，避免录入错人。
func (s *FaceVerifyService) AddFaceFromCamera(
	ctx context.Context,
	name string,
	cam camera.Camera,
) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("name cannot be empty")
	}

	if cam == nil {
		return 0, ErrNilCamera
	}

	exists, err := facedb.FaceExistsByName(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("check face exists by name: %w", err)
	}

	if exists {
		return 0, ErrFaceAlreadyExists
	}

	embedding, err := s.ExtractSingleEmbeddingFromCamera(ctx, cam)
	if err != nil {
		return 0, fmt.Errorf("extract single embedding from camera: %w", err)
	}

	embeddingText := EmbeddingToPGVector(embedding)

	id, err := facedb.AddFace(ctx, name, embeddingText)
	if err != nil {
		return 0, fmt.Errorf("add face to database: %w", err)
	}

	return id, nil
}

func (s *FaceVerifyService) DeleteFaceByName(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name cannot be empty")
	}

	if err := facedb.DeleteFaceByName(ctx, name); err != nil {
		return fmt.Errorf("delete face by name: %w", err)
	}

	return nil
}

func (s *FaceVerifyService) QueryFaceByName(ctx context.Context, name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, errors.New("name cannot be empty")
	}

	exists, err := facedb.FaceExistsByName(ctx, name)
	if err != nil {
		return false, fmt.Errorf("query face by name: %w", err)
	}

	return exists, nil
}

// ExtractSingleEmbeddingFromCamera 给添加人脸用
// 如果图片里有多张脸，直接报错，避免录入错人
func (s *FaceVerifyService) ExtractSingleEmbeddingFromCamera(
	ctx context.Context,
	cam camera.Camera,
) ([]float64, error) {
	return s.extractEmbeddingFromCamera(ctx, cam, true)
}
