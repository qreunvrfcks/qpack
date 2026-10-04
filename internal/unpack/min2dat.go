package unpack

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"qpack/internal/config"
	"qpack/internal/logger"
	"qpack/internal/pack"
	"strings"
)

// Обратная к RawToPlate: номер платы (0-17) -> номер ряда в data (0-21) с учетом плейсхолдеров.
func plateToRaw() [config.NPlates]int {
	var m [config.NPlates]int
	for pos, idx := range config.RawToPlate {
		if idx >= 0 {
			m[idx] = pos
		}
	}
	return m
}

// Разворачивает все .bin ниже inputRoot в .dat ниже outputRoot.
// Относительные пути и имена сохраняются (.bin -> .dat).
// Битый файл пропускается с READ-ошибкой в лог и консоль.
func UnpackDir(inputRoot, outputRoot string) (int, error) {
	count := 0
	err := filepath.WalkDir(inputRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".min") {
			return nil
		}

		relativePath, err := filepath.Rel(inputRoot, path)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(outputRoot, strings.TrimSuffix(relativePath, filepath.Ext(relativePath))+".dat")
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}
		if err := UnpackFile(path, outputPath); err != nil {
			logger.Error("READ %s: %v (skipped)", path, err)
			os.Remove(outputPath) // недожатый хвост не оставляем
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Разворачивает файл .bin в .dat.
func UnpackFile(inputFile, outputFile string) error {
	axis, err := pack.AxisCode(inputFile)
	if err != nil {
		return err
	}
	ySide := axis != 0

	events, err := DecodeMin(inputFile)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if ev.Y != ySide {
			return fmt.Errorf("axis mismatch in %q (file says y=%v)", inputFile, ev.Y)
		}
	}

	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", outputFile, err)
	}
	defer out.Close()

	buf := bufio.NewWriter(out)
	p2r := plateToRaw()
	var sb strings.Builder
	for n, ev := range events {
		sb.Reset()
		// trig: код*100+55 (55->55, 1->155, ..., 15->1555); время назад в t_sec/t_abs
		fmt.Fprintf(&sb, "%d\t%d\t%d\t%d\t%d",
			ev.Trig*100+55, n+1, ev.Delta, uint32(ev.Time%uint64(config.NSPerSec)), uint32(ev.Time/uint64(config.NSPerSec)))
		var raw [config.DataLen]uint32
		for idx, v := range ev.Data {
			raw[p2r[idx]] = v
		}
		for _, v := range raw {
			fmt.Fprintf(&sb, "\t%x", v)
		}
		sb.WriteString("\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\n")
		if _, err := buf.WriteString(sb.String()); err != nil {
			return fmt.Errorf("write event %d to %q: %w", n+1, outputFile, err)
		}
	}
	if err := buf.Flush(); err != nil {
		return fmt.Errorf("flush %q: %w", outputFile, err)
	}
	return nil
}
