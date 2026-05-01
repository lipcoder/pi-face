package recognition

import (
	"fmt"
	"net/http"

	"lipcoder/face/internal/adapter/camera/local"
	"lipcoder/face/internal/adapter/inspireface"
	facejson "lipcoder/face/internal/domain/json"
)

// ExtractEmbeddingFromLocalCamera 不保存图片：
// 1. 从本地摄像头获取照片
// 2. 发送到 InspireFace 服务端
// 3. 通过 domain/json 包解析返回值
// 4. 只返回 Embedding
func ExtractEmbeddingFromLocalCamera(httpClient *http.Client) ([]float64, error) {
	cam := &local.Local{}

	imageBytes, err := cam.Capture()
	if err != nil {
		return nil, fmt.Errorf("capture local camera image: %w", err)
	}

	insp := inspireface.NewInspire(httpClient)

	respBody, err := insp.PostImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("post image to inspireface: %w", err)
	}

	embedding, err := facejson.GetBestFaceEmbedding(respBody)
	if err != nil {
		return nil, fmt.Errorf("get best face embedding: %w", err)
	}

	return embedding, nil
}