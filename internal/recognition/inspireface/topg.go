package inspireface

import (
	"strconv"
	"strings"
)

func (a Inspire)EmbeddingToPGVector(embedding []float64) string {
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
