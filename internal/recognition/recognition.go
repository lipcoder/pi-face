package recognition

type Recognition interface {
	PostImage(imgBytes []byte) ([]byte, error)
	GetFaceEmbedding(respBody []byte, rank int) ([][]float64, error)
	EmbeddingToPGVector(embedding []float64) string
}
