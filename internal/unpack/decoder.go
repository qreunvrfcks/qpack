package unpack

// Декодер архивного min-формата v12 (бит-в-бит с muon/internal/convertor).
// Нужен только для unpack: .min -> .dat построчно.

import (
	"encoding/binary"
	"fmt"
	"os"
	"qpack/internal/config"
)

// bitReader читает биты LSB-first курсором по слайсу.
type bitReader struct {
	buf []byte
	pos uint // битовая позиция от начала buf
}

func (b *bitReader) get(width uint) uint64 {
	var v uint64
	for i := uint(0); i < width; i++ {
		p := b.pos + i
		if (b.buf[p>>3]>>(p&7))&1 != 0 {
			v |= 1 << i
		}
	}
	b.pos += width
	return v
}

func (b *bitReader) left() int { return len(b.buf)*8 - int(b.pos) }

// MinEvent — одно декодированное событие: ось, время, слова плат, delta, код триггера.
type MinEvent struct {
	Y     bool
	Time  uint64
	Delta uint64
	Trig  uint64
	Data  [config.NPlates]uint32
}

// DecodeMin читает весь min-файл в память.
func DecodeMin(path string) ([]MinEvent, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	if len(buf) < config.HeaderLen {
		return nil, fmt.Errorf("truncated header in %q", path)
	}
	if magic := binary.LittleEndian.Uint32(buf[0:]); magic != config.Magic {
		return nil, fmt.Errorf("bad magic %x in %q", magic, path)
	}
	if ver := binary.LittleEndian.Uint32(buf[4:]); ver != config.ArchiveVersion {
		return nil, fmt.Errorf("bad version %d in %q, want %d", ver, path, config.ArchiveVersion)
	}
	if layers := binary.LittleEndian.Uint32(buf[8:]); layers != uint32(config.Layers) {
		return nil, fmt.Errorf("bad layers %d in %q, config has %d", layers, path, config.Layers)
	}
	if plates := binary.LittleEndian.Uint32(buf[12:]); plates != uint32(config.PlatesPerLayer) {
		return nil, fmt.Errorf("bad plates %d in %q, config has %d", plates, path, config.PlatesPerLayer)
	}

	br := &bitReader{buf: buf[config.HeaderLen:]}
	var events []MinEvent
	var data [config.NPlates]uint32
	var prev uint64
	var haveBase bool

	for rowNo := 1; ; rowNo++ {
		if br.left() == 0 {
			break
		}
		if br.left() < int(config.MarkerBits+config.TDeltaVBit+1) {
			if v := br.get(uint(br.left())); v != 0 {
				return nil, fmt.Errorf("row %d: nonzero tail pad in %q", rowNo, path)
			}
			break
		}

		marker := br.get(config.MarkerBits)
		y := marker&1 != 0
		trig := marker >> 1

		tLen := br.get(config.TDeltaVBit)
		var t uint64
		if !haveBase {
			if tLen != 0 {
				return nil, fmt.Errorf("row %d in %q: first tLen=%d, want 0", rowNo, path, tLen)
			}
			t = br.get(config.TDeltaBits)
			prev, haveBase = t, true
		} else {
			if tLen == 0 || tLen > uint64(config.TDeltaBits) {
				return nil, fmt.Errorf("row %d in %q: bad tLen %d", rowNo, path, tLen)
			}
			rest := uint64(0)
			if tLen > 1 {
				rest = br.get(uint(tLen - 1))
			}
			t = prev + (uint64(1)<<(tLen-1) | rest)
			prev = t
		}
		delta := br.get(config.DeltaBits)

		for i := range data {
			data[i] = 0
		}
		for {
			cont := br.get(1)
			if cont == 0 {
				break
			}
			idx := br.get(config.IdxBits)
			cnt := br.get(config.CntBits) + 1
			if int(idx) >= config.NPlates {
				return nil, fmt.Errorf("row %d: bad idx %d in %q", rowNo, idx, path)
			}
			if uint(br.left()) < uint(cnt)*config.KbBits {
				return nil, fmt.Errorf("row %d: truncated hits in %q", rowNo, path)
			}
			for range cnt {
				kb := br.get(config.KbBits)
				j, k := kb>>4, kb&0xf
				if j >= uint64(config.RowsPerPlate) {
					return nil, fmt.Errorf("row %d: bad j %d in %q", rowNo, j, path)
				}
				data[idx] |= 1 << (j*uint64(config.BinsPerRow) + k)
			}
		}

		events = append(events, MinEvent{Y: y, Time: t, Delta: delta, Trig: trig, Data: data})
	}

	return events, nil
}
