package inspireface

import (
	"errors"
	"fmt"
)

func (a Inspire) GetFaceEmbedding(imageBytes []byte, rank int) ([][]float64, error) {
	respBody, err := a.PostImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("mainCycle post image failed,%w", err)
	}
	response, err := BytesFromResponse(respBody)
	if err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, errors.New("inspireface response ok is false")
	}
	if len(response.Faces) == 0 {
		return nil, errors.New("no face detected")
	}
	switch rank {
	case -1:
		// 返回所有人脸 embedding
		embeddings := make([][]float64, 0, len(response.Faces))

		for i, face := range response.Faces {
			if len(face.Embedding) == 0 {
				return nil, fmt.Errorf("face %d embedding is empty", i)
			}

			embeddings = append(embeddings, face.Embedding)
		}

		return embeddings, nil
	case 1:
		// 返回质量最高的一张脸
		bestFace := response.Faces[0]

		for _, face := range response.Faces[1:] {
			if face.Quality > bestFace.Quality {
				bestFace = face
			}
		}

		if len(bestFace.Embedding) == 0 {
			return nil, errors.New("best face embedding is empty")
		}

		return [][]float64{bestFace.Embedding}, nil
	default:
		return nil, fmt.Errorf("unsupported rank: %d", rank)
	}
}
