package pack

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"qpack/internal/config"
	"strings"
)

// Пакует runDir в <runDir>.tar.gz рядом.
// Возвращает путь к архиву.
func PackRun(runDir string) (string, error) {
	arcPath := strings.TrimSuffix(runDir, string(filepath.Separator)) + ".tar.gz"
	if _, err := WriteTar(runDir, arcPath); err != nil {
		return "", err
	}
	return arcPath, nil
}

// Жмёт все .dat ниже inputRoot в архив arcPath:
// Внутри архива run-папка <runName>/ (.qpac + yaml + отпечаток).
func PackDirTo(inputRoot, arcPath, runName string) (int, error) {
	stage, err := os.MkdirTemp("", "qpack-*")
	if err != nil {
		return 0, fmt.Errorf("mktemp stage: %w", err)
	}
	defer os.RemoveAll(stage)
	runDir := filepath.Join(stage, runName)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return 0, fmt.Errorf("mkdir stage run dir: %w", err)
	}
	count, err := convertInto(inputRoot, runDir)
	if err != nil {
		return 0, err
	}
	if err := config.WriteStamp(runDir, "qpack"); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(arcPath), 0755); err != nil {
		return 0, fmt.Errorf("mkdir archive dir: %w", err)
	}
	if _, err := WriteTar(runDir, arcPath); err != nil {
		return 0, err
	}
	return count, nil
}

// Пакует runDir в архив arcPath (имена внутри от runName).
func WriteTar(runDir, arcPath string) (string, error) {
	out, err := os.Create(arcPath)
	if err != nil {
		return "", fmt.Errorf("create archive %q: %w", arcPath, err)
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("gzip writer: %w", err)
	}
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	base := filepath.Dir(runDir)
	err = filepath.WalkDir(runDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("tar header %q: %w", rel, err)
		}
		if entry.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("tar file %q: %w", rel, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	return arcPath, nil
}
