package unpack

// Распаковка .tar.gz архива run-папки во временную папку.
// Архив — только переноска: muon и unpack работают с распакованной папкой.

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnpackTar разархивирует .tar.gz во временную папку, возвращает её путь.
// Вызывающий обязан удалить папку (defer os.RemoveAll).
func UnpackTar(arcPath string) (string, error) {
	f, err := os.Open(arcPath)
	if err != nil {
		return "", fmt.Errorf("open archive %q: %w", arcPath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip open %q: %w", arcPath, err)
	}
	defer gz.Close()

	stage, err := os.MkdirTemp("", "qpack-unpack-*")
	if err != nil {
		return "", fmt.Errorf("mktemp stage: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(stage)
			return "", fmt.Errorf("tar read %q: %w", arcPath, err)
		}
		dst := filepath.Join(stage, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(dst, stage) {
			os.RemoveAll(stage)
			return "", fmt.Errorf("tar entry escapes stage: %q", hdr.Name)
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0755); err != nil {
				os.RemoveAll(stage)
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			os.RemoveAll(stage)
			return "", err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			os.RemoveAll(stage)
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			os.RemoveAll(stage)
			return "", fmt.Errorf("tar extract %q: %w", hdr.Name, err)
		}
		out.Close()
	}
	return stage, nil
}
