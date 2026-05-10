package inspireface

import (
	"errors"
	"fmt"
	"lipcoder/face/internal/recognition"
)

func (a Inspire) GetFaceEmbedding(imageBytes []byte, rank int) ([][]float64, error) {
	respBody, err := a.PostImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("post image failed: %w", err)
	}
	response, err := BytesFromResponse(respBody)
	if err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, errors.New("inspireface response ok is false")
	}
	if response.FaceCount == 0 || len(response.Faces) == 0 {
		return nil, recognition.ErrNoFace
	}
	switch rank {
	case -1:
		// 返回所有人脸 embedding
		embeddings := make([][]float64, 0, len(response.Faces))

		for _, face := range response.Faces {
			if len(face.Embedding) == 0 {
				return nil, recognition.ErrNoFaceEmbedding
			}

			embeddings = append(embeddings, face.Embedding)
		}

		return embeddings, nil
	case 0:
		// 只允许图片里有一张脸
		if len(response.Faces) != 1 {
			return nil, recognition.ErrNotOneFace
		}

		embedding := response.Faces[0].Embedding
		if len(embedding) == 0 {
			return nil, recognition.ErrNoFaceEmbedding
		}

		return [][]float64{embedding}, nil
	case 1:
		// 返回质量最高的一张脸
		bestFace := response.Faces[0]

		if len(bestFace.Embedding) == 0 {
			return nil, recognition.ErrNoFaceEmbedding
		}

		return [][]float64{bestFace.Embedding}, nil
	default:
		return nil, fmt.Errorf("unsupported rank: %d", rank)
	}
}
