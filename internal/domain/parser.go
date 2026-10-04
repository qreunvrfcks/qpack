package domain

// Общий парсер одной строки .dat. Писатели dat->min и dat->db (пакет merge)
// пользуются только им. Place-хвост и номер события не парсятся —
// они никуда не пишутся.

import (
	"fmt"
	"path/filepath"
	"qpack/internal/config"
	"strconv"
	"strings"
)

// DatRow — распарсенная строка .dat: только то, что пишется в файлы.
// Время — одним числом: младшие разряды из t_abs + старшие из t_sec
// (без умножения на NSPerSec: оба значения уже в тиках АЦП 1e8/с).
// Trig — код триггера = trig/100 (55->0, 155->1, ..., 1555->15).
// Сырое всегда вида ...55. В конфиге не задаётся.
type DatRow struct {
	Trig  int32
	Delta uint64
	Time  uint64
	Raw   [config.DataLen]uint32
}

// AxisCode — бит оси по имени файла: _X_ -> 0, _Y_ -> 1<<7.
func AxisCode(inputFile string) (uint8, error) {
	name := strings.ToUpper(filepath.Base(inputFile))
	switch {
	case strings.Contains(name, strings.ToUpper(config.XPattern)):
		return 0, nil
	case strings.Contains(name, strings.ToUpper(config.YPattern)):
		return 1 << 7, nil
	default:
		return 0, fmt.Errorf("cannot detect axis (want %q or %q) in file name %q",
			config.XPattern, config.YPattern, filepath.Base(inputFile))
	}
}

func ParseDatLine(fields []string, lineNo int, inputFile string) (DatRow, error) {
	var r DatRow

	trig, err := strconv.ParseInt(fields[config.TrigCol], 10, 32)
	if err != nil {
		return r, fmt.Errorf("line %d in %q: parse trig %q: %w", lineNo, inputFile, fields[config.TrigCol], err)
	}
	if trig%100 != 55 || trig < 0 || trig > 1555 {
		return r, fmt.Errorf("line %d in %q: bad trigger %d (want ...55, 0..1555)", lineNo, inputFile, trig)
	}
	r.Trig = int32(trig / 100) // 55->0, 155->1, ..., 1555->15

	delta, err := strconv.ParseUint(fields[config.DeltaCol], 10, 32)
	if err != nil {
		return r, fmt.Errorf("line %d in %q: parse delta %q: %w", lineNo, inputFile, fields[config.DeltaCol], err)
	}
	maxD := uint64(1)<<config.DeltaBits - 1
	if delta > maxD {
		return r, fmt.Errorf("line %d in %q: delta %d exceeds %d bits", lineNo, inputFile, delta, config.DeltaBits)
	}
	r.Delta = delta
	tAbs, err := strconv.ParseUint(fields[config.TAbsCol], 10, 32)
	if err != nil {
		return r, fmt.Errorf("line %d in %q: parse t_abs %q: %w", lineNo, inputFile, fields[config.TAbsCol], err)
	}

	tSec, err := strconv.ParseUint(fields[config.TSecCol], 10, 32)
	if err != nil {
		return r, fmt.Errorf("line %d in %q: parse t_sec %q: %w", lineNo, inputFile, fields[config.TSecCol], err)
	}
	r.Time = tSec<<32 | tAbs

	for i, token := range fields[config.DataOffset : config.DataOffset+config.DataLen] {
		parsed, err := strconv.ParseUint(token, 16, 32)
		if err != nil {
			return r, fmt.Errorf("line %d col %d in %q: parse %q as hex uint32: %w", lineNo, config.DataOffset+i+1, inputFile, token, err)
		}
		r.Raw[i] = uint32(parsed)
	}

	return r, nil
}
