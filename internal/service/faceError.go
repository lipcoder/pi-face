package recognition

import (
	"errors"
)

var (
	ErrNilCamera         = errors.New("camera cannot be nil")
	ErrImageEmpty        = errors.New("image is empty")
	ErrNoFaceDetected    = errors.New("no face detected")
	ErrMultipleFaces     = errors.New("multiple faces detected")
	ErrEmbeddingEmpty    = errors.New("embedding is empty")
	ErrFaceAlreadyExists = errors.New("face already exists")
)
