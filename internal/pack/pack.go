package pack

// Архивный конвертер: тот же .dat -> .bin, но биты ужаты в ноль.
// Только для хранения. Параметры — из qpack/internal/

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"qpack/internal/config"
	"qpack/internal/logger"
	"strings"
)

// bitWriter льёт биты LSB-first в bufio.Writer. Ноль аллокаций.
type bitWriter struct {
	w   *bufio.Writer
	acc uint64
	n   uint
	err error
}

func (b *bitWriter) put(v uint64, width uint) {
	if b.err != nil {
		return
	}
	b.acc |= v << b.n
	b.n += width
	for b.n >= 8 {
		if werr := b.w.WriteByte(byte(b.acc)); werr != nil {
			b.err = werr
			return
		}
		b.acc >>= 8
		b.n -= 8
	}
}

func (b *bitWriter) flush(path string) error {
	if b.err != nil {
		return b.err
	}
	if b.n > 0 {
		if err := b.w.WriteByte(byte(b.acc)); err != nil {
			return fmt.Errorf("write tail to %q: %w", path, err)
		}
	}
	if err := b.w.Flush(); err != nil {
		return fmt.Errorf("flush %q: %w", path, err)
	}
	return nil
}

// plateGroup — одна непустая плата.
type plateGroup struct {
	idx uint8
	cnt uint8
	kb  [config.HitsPerPlate]uint8
}

// ConvertDir жмёт все .dat ниже inputRoot в run-папку ниже outputRoot:
// <outputRoot>/<runName>/ — .min файлы (расширение .dat -> .min),
// detector.qpack.yaml + fingerprint.txt рядом. Имя runName задаёт вызывающий.
func ConvertDir(inputRoot, outputRoot, runName string) (string, int, error) {
	runDir := filepath.Join(outputRoot, runName)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return "", 0, fmt.Errorf("mkdir run dir %q: %w", runDir, err)
	}
	count, err := convertInto(inputRoot, runDir)
	if err != nil {
		return "", 0, err
	}
	if err := config.WriteStamp(runDir, "qpack"); err != nil {
		return "", 0, err
	}
	return runDir, count, nil
}

// convertInto — общая проходка .dat -> .min, пишет в готовую папку runDir.
// Битый файл (ошибка чтения/парса строки) скипается: ошибка идёт в лог
// и в консоль через logger, проход продолжается. Возвращает число сжатых.
func convertInto(inputRoot, runDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(inputRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".dat") {
			return nil
		}

		relativePath, err := filepath.Rel(inputRoot, path)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(runDir, strings.TrimSuffix(relativePath, filepath.Ext(relativePath))+".min")
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}
		if err := ConvertFileMin(path, outputPath); err != nil {
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

// ConvertFileMin конвертирует один .dat в один архивный .min v12.
// Расклад события: [marker:3][tLen:4][tAbs(первое) | tDelta][delta:25]
// [группы idx:5/cnt-1:5/kb:5]. Пустые события (все платы нулевые) скипаются.
// Время хранится дельтами от базового (первое событие); дельта с varint:
// длина tLen (бит, минимум 1), значение без старшего бита следом.
func ConvertFileMin(inputFile, outputFile string) error {
	axis, err := AxisCode(inputFile)
	if err != nil {
		return err
	}
	var axisBit uint64
	if axis != 0 {
		axisBit = 1
	}

	in, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("open input file %q: %w", inputFile, err)
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", outputFile, err)
	}
	defer out.Close()

	buffered := bufio.NewWriter(out)
	for _, word := range []uint32{config.Magic, config.ArchiveVersion, uint32(config.Layers), uint32(config.PlatesPerLayer)} {
		if err := binary.Write(buffered, binary.LittleEndian, word); err != nil {
			return fmt.Errorf("write header to %q: %w", outputFile, err)
		}
	}
	bw := &bitWriter{w: buffered}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var groups [config.NPlates]plateGroup
	var prev uint64
	var haveBase bool

	putTime := func(t uint64) {
		if !haveBase {
			bw.put(0, config.TDeltaVBit) // tLen=0 → дальше абсолютное время
			bw.put(t, config.TDeltaBits)
			prev, haveBase = t, true
			return
		}
		delta := t - prev
		prev = t
		n := uint(64 - bits.LeadingZeros64(delta|1))
		bw.put(uint64(n), config.TDeltaVBit)
		bw.put(delta^(uint64(1)<<(n-1)), n-1)
	}

	firstData := true
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '!' || line[0] == '#' {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != config.NumCols {
			if !firstData {
				logger.Error("ROW %s line %d: got %d fields, want %d (skipped)", inputFile, lineNo, len(fields), config.NumCols)
			}
			firstData = false
			continue
		}

		row, err := ParseDatLine(fields, lineNo, inputFile)
		if err != nil {
			if !firstData {
				logger.Error("ROW %s line %d: %v (skipped)", inputFile, lineNo, err)
			}
			firstData = false
			continue
		}
		firstData = false
		arcIdx := uint64(row.Trig)
		raw := row.Raw
		// Калибровка: в разделителях что-то есть — в лог, из файла выкинуть.
		var sepAny uint32
		var sepVals [4]uint32
		for s, pos := range config.Spaceholders {
			sepVals[s] = raw[pos]
			sepAny |= raw[pos]
		}
		if sepAny != 0 {
			logger.Printf("SKIP %s line %d: config.Spaceholders %x", inputFile, lineNo, sepVals)
			continue
		}

		ng := 0
		for pos, v := range raw {
			idx := config.RawToPlate[pos]
			if idx < 0 || v == 0 {
				continue
			}
			g := &groups[ng]
			g.idx = uint8(idx)
			n := 0
			for j, half := range [2]uint16{uint16(v & 0xffff), uint16(v >> 16)} {
				h := half
				for h != 0 {
					k := bits.TrailingZeros16(h)
					g.kb[n] = uint8(j<<4 | k)
					n++
					h &= h - 1
				}
			}
			g.cnt = uint8(n)
			ng++
		}
		if ng == 0 {
			continue // пустое событие: плат нет, писать нечего
		}

		bw.put(axisBit|(arcIdx<<1), config.MarkerBits)
		putTime(row.Time)
		bw.put(row.Delta, config.DeltaBits)
		for _, g := range groups[:ng] {
			bw.put(1, 1) // продолжение групп; 0 — конец (см. декодер)
			bw.put(uint64(g.idx), config.IdxBits)
			bw.put(uint64(g.cnt)-1, config.CntBits)
			for _, kb := range g.kb[:g.cnt] {
				bw.put(uint64(kb), config.KbBits)
			}
		}
		bw.put(0, 1) // конец групп события
		if bw.err != nil {
			return fmt.Errorf("write event (line %d) to %q: %w", lineNo, outputFile, bw.err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %q: %w", inputFile, err)
	}

	return bw.flush(outputFile)
}
