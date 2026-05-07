package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var attendanceRecordMu sync.Mutex

func RecordFaceSimilarity(name string, faceSimilarity float64) error {
	attendanceRecordMu.Lock()
	defer attendanceRecordMu.Unlock()

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}

	now := time.Now().In(loc)

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	dataDir := filepath.Join(wd, "data")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	fileName := now.Format("2006-01-02") + ".txt"
	filePath := filepath.Join(dataDir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	line := fmt.Sprintf(
		"%s %s %.6f\n",
		now.Format("15:04"),
		name,
		faceSimilarity,
	)

	_, err = file.WriteString(line)
	return err
}