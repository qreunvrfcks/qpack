package config

// Параметры детектора из default.yaml рядом (embed, грузится в init).
// Значения не меняются — просто живут в одном месте, код берёт их отсюда
// вместо захардкоженных констант. Всё выведенное (таблицы, ширины бит)
// строится здесь же, конвертер только пользуется.
//
// Внимание: это копия muon/internal/config, урезанная под dat->min:
// оставлены только секции detector/dat_format/axis/triggers/archive_format.
// Секции optimal_format/merge здесь не нужны и не читаются.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var rawYAML []byte

// Размеры массивов должны быть константами (буферы рядов).
// Дублируют yaml один в один; при смене yaml поменять и их.
const (
	NPlates      = 18
	DataLen      = 22
	HitsPerPlate = 32
)

type fileYAML struct {
	Detector struct {
		Layers         int   `yaml:"layers"`
		PlatesPerLayer int   `yaml:"plates_per_layer"`
		RowsPerPlate   int   `yaml:"rows_per_plate"`
		BinsPerRow     int   `yaml:"bins_per_row"`
		Spaceholders   []int `yaml:"spaceholders"`
	} `yaml:"detector"`
	DatFormat struct {
		NumCols    int    `yaml:"num_cols"`
		HeaderCols int    `yaml:"header_cols"`
		TrigCol    int    `yaml:"trig_col"`
		DeltaCol   int    `yaml:"delta_col"`
		TAbsCol    int    `yaml:"tabs_col"`
		TSecCol    int    `yaml:"tsec_col"`
		DataOffset int    `yaml:"data_offset"`
		DataLen    int    `yaml:"data_len"`
		NSPerSec   uint64 `yaml:"ns_per_sec"`
	} `yaml:"dat_format"`
	Axis struct {
		XPattern string `yaml:"x_pattern"`
		YPattern string `yaml:"y_pattern"`
	} `yaml:"axis"`
	ArchiveFormat struct {
		Magic      uint32 `yaml:"magic"`
		Version    uint32 `yaml:"version"`
		MarkerBit  uint   `yaml:"marker_bits"`
		DeltaBits  uint   `yaml:"delta_bits"`
		TDeltaBits uint   `yaml:"tdelta_bits"`
		TDeltaVBit uint   `yaml:"tdelta_vbits"`
		IdxBits    uint   `yaml:"idx_bits"`
		CntBits    uint   `yaml:"cntMinus1_bits"`
		KbBits     uint   `yaml:"kb_bits"`
	} `yaml:"archive_format"`
}

var (
	// Source — откуда взят активный конфиг: "embed" или путь к файлу.
	Source = "embed"
	// activeYAML — байты активного конфига (для штампа и отпечатка).
	activeYAML     = rawYAML
	mu             sync.Mutex
	Layers         int
	PlatesPerLayer int
	RowsPerPlate   int
	BinsPerRow     int
	Spaceholders   []int
	RawToPlate     []int // сырая позиция в блоке data -> плотный idx платы, -1 = разделитель

	NumCols    int
	HeaderCols int
	TrigCol    int
	DeltaCol   int
	TAbsCol    int
	TSecCol    int
	DataOffset int
	NSPerSec   uint64

	XPattern string
	YPattern string

	Magic          uint32
	HeaderLen      = 16
	ArchiveVersion uint32
	MarkerBits     uint
	DeltaBits      uint
	TDeltaBits     uint
	TDeltaVBit     uint
	IdxBits        uint
	CntBits        uint
	KbBits         uint
)

func init() {
	apply(parse(rawYAML))
}

func parse(data []byte) fileYAML {
	var f fileYAML
	if err := yaml.Unmarshal(data, &f); err != nil {
		panic(fmt.Sprintf("config: parse default.yaml: %v", err))
	}
	return f
}

// DefaultBytes возвращает встроенный default.yaml по умолчанию.
func DefaultBytes() []byte { return rawYAML }

// WriteDefault пишет встроенный конфиг в path (родители создаются).
func WriteDefault(path string) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir config dir: %w", err)
		}
	}
	if err := os.WriteFile(path, rawYAML, 0644); err != nil {
		return fmt.Errorf("write default config %q: %w", path, err)
	}
	return nil
}

// LoadFile подменяет активный конфиг внешним yaml-файлом.
// Вызывать до конвертации (после init, который грузит встроенный).
func LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	mu.Lock()
	defer mu.Unlock()
	apply(parse(data))
	activeYAML = data
	Source = path
	return nil
}

// Fingerprint — sha256 hex от АКТИВНОГО конфига (встроенного или из файла).
func Fingerprint() string {
	mu.Lock()
	defer mu.Unlock()
	sum := sha256.Sum256(activeYAML)
	return hex.EncodeToString(sum[:])
}

// WriteStamp пишет рядом с выходом копию АКТИВНОГО конфига и строку в журнал:
// <dir>/detector.<tool>.yaml + <dir>/fingerprint.txt ("<sha> <tool> <UTC RFC3339>").
func WriteStamp(dir, tool string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir stamp dir %q: %w", dir, err)
	}
	dst := filepath.Join(dir, "detector."+tool+".yaml")
	mu.Lock()
	data := activeYAML
	mu.Unlock()
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("write stamp %q: %w", dst, err)
	}
	line := fmt.Sprintf("%s %s %s\n", Fingerprint(), tool, time.Now().UTC().Format(time.RFC3339))
	f, err := os.OpenFile(filepath.Join(dir, "fingerprint.txt"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open fingerprint journal: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write fingerprint journal: %w", err)
	}
	return nil
}

func apply(f fileYAML) {
	Layers = f.Detector.Layers
	PlatesPerLayer = f.Detector.PlatesPerLayer
	RowsPerPlate = f.Detector.RowsPerPlate
	BinsPerRow = f.Detector.BinsPerRow

	Spaceholders = f.Detector.Spaceholders

	NumCols = f.DatFormat.NumCols
	HeaderCols = f.DatFormat.HeaderCols
	TrigCol = f.DatFormat.TrigCol
	DeltaCol = f.DatFormat.DeltaCol
	TAbsCol = f.DatFormat.TAbsCol
	TSecCol = f.DatFormat.TSecCol
	DataOffset = f.DatFormat.DataOffset
	NSPerSec = f.DatFormat.NSPerSec

	XPattern = f.Axis.XPattern
	YPattern = f.Axis.YPattern

	Magic = f.ArchiveFormat.Magic
	ArchiveVersion = f.ArchiveFormat.Version
	MarkerBits = f.ArchiveFormat.MarkerBit
	DeltaBits = f.ArchiveFormat.DeltaBits
	TDeltaBits = f.ArchiveFormat.TDeltaBits
	TDeltaVBit = f.ArchiveFormat.TDeltaVBit
	IdxBits = f.ArchiveFormat.IdxBits
	CntBits = f.ArchiveFormat.CntBits
	KbBits = f.ArchiveFormat.KbBits

	// Разделители режут блок data на плотные индексы плат по порядку.
	seps := map[int]bool{}
	for _, s := range Spaceholders {
		seps[s] = true
	}
	RawToPlate = make([]int, f.DatFormat.DataLen)
	plate := 0
	for pos := range RawToPlate {
		if seps[pos] {
			RawToPlate[pos] = -1
			continue
		}
		RawToPlate[pos] = plate
		plate++
	}
}
