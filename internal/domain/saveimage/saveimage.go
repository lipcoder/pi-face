package saveimage

import (
	"fmt"
	"os"
	"path/filepath"
)

func SaveImage(filePath string, imageBytes []byte) error {
	if len(imageBytes) == 0 {
		return fmt.Errorf("图片数据为空")
	}

	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(filePath, imageBytes, 0644); err != nil {
		return fmt.Errorf("保存图片失败: %w", err)
	}

	return nil
}
