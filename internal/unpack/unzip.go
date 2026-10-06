package unpack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Разархивирует .zip во временную папку.
// Возвращает корень stage (удалять через defer) и папку с содержимым:
// если в архиве единственный корневой каталог (run-папка pack) — спускается в него.
func UnpackZip(arcPath string) (root, content string, err error) {
	zr, err := zip.OpenReader(arcPath)
	if err != nil {
		return "", "", fmt.Errorf("zip open %q: %w", arcPath, err)
	}
	defer zr.Close()

	stage, err := os.MkdirTemp("", "qpack-unpack-*")
	if err != nil {
		return "", "", fmt.Errorf("mktemp stage: %w", err)
	}

	for _, f := range zr.File {
		dst := filepath.Join(stage, filepath.Clean(f.Name))
		if !strings.HasPrefix(dst, stage) {
			os.RemoveAll(stage)
			return "", "", fmt.Errorf("zip entry escapes stage: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0755); err != nil {
				os.RemoveAll(stage)
				return "", "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			os.RemoveAll(stage)
			return "", "", err
		}
		rc, err := f.Open()
		if err != nil {
			os.RemoveAll(stage)
			return "", "", err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			os.RemoveAll(stage)
			return "", "", err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			os.RemoveAll(stage)
			return "", "", fmt.Errorf("zip extract %q: %w", f.Name, err)
		}
		rc.Close()
		out.Close()
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		os.RemoveAll(stage)
		return "", "", fmt.Errorf("read stage %q: %w", arcPath, err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return stage, filepath.Join(stage, entries[0].Name()), nil
	}
	return stage, stage, nil
}
