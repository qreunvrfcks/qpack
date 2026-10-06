package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"qpack/internal/config"
)

// Пакует runDir в архив arcPath (.zip, имена внутри от runName).
func WriteZip(runDir, arcPath string) (string, error) {
	out, err := os.Create(arcPath)
	if err != nil {
		return "", fmt.Errorf("create archive %q: %w", arcPath, err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	base := filepath.Dir(runDir)
	err = filepath.WalkDir(runDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel == "." {
				return nil
			}
			_, err := zw.Create(rel + "/")
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return fmt.Errorf("zip header %q: %w", rel, err)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return fmt.Errorf("zip file %q: %w", rel, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("zip close: %w", err)
	}
	return arcPath, nil
}

// Жмёт все .dat ниже inputRoot в архив arcPath (.zip):
// внутри архива run-папка <runName>/ (.qpac + yaml + отпечаток).
func PackDirToZip(inputRoot, arcPath, runName string) (int, error) {
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
	if _, err := WriteZip(runDir, arcPath); err != nil {
		return 0, err
	}
	return count, nil
}
