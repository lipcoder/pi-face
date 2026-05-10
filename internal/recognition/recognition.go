package recognition

import "errors"

var (
	ErrNoFace          = errors.New("face is empty")
	ErrNotOneFace      = errors.New("not only one face")
	ErrNoFaceEmbedding = errors.New("face embedding is empty")
)

type Recognition interface {
	PostImage(imgBytes []byte) ([]byte, error)
	GetFaceEmbedding(respBody []byte, rank int) ([][]float64, error)
}
