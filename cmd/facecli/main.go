package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	recognition "lipcoder/face/internal/service/face"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	embedding, err := recognition.ExtractEmbeddingFromLocalCamera(&http.Client{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		logger.Error("extract embedding from local camera failed", "err", err)
		os.Exit(1)
	}

	output, err := json.Marshal(embedding)
	if err != nil {
		logger.Error("marshal embedding failed", "err", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}