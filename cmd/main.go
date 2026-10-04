package main

// qpack: .dat -> (.bin) для хранения.

import (
	"fmt"
	"os"
	"path/filepath"
	"qpack/internal/config"
	"qpack/internal/logger"
	"qpack/internal/pack"
	"qpack/internal/unpack"
	"strings"
)

func usage() {
	fmt.Fprintln(os.Stderr, "  qpack pack <inDir> <outDir> [--run <name>] [--config <path.yaml>] [--tar]  # .dat -> run-папка (.min + yaml + отпечаток)")
	fmt.Fprintln(os.Stderr, "  qpack unpack <inDir|in.tar.gz> <outDir> [--config <path.yaml>]  # .min или архив -> .dat (n с 1, разделители и place — нули)")
	fmt.Fprintln(os.Stderr, "  qpack init-config [path]             # записать встроенный конфиг (по умолч. default.yaml)")
}

// Отделяет позиционные от флагов --config/--run/--tar (в любом порядке).
func splitArgs(args []string) (pos []string, cfgPath, runName string, cfgSet, runSet, tarSet bool) {
	skip := map[int]bool{}
	for i := range args {
		a := args[i]
		if a == "--config" && i+1 < len(args) {
			cfgPath, cfgSet, skip[i], skip[i+1] = args[i+1], true, true, true
		} else if rest, ok := strings.CutPrefix(a, "--config="); ok {
			cfgPath, cfgSet, skip[i] = rest, true, true
		} else if a == "--run" && i+1 < len(args) {
			runName, runSet, skip[i], skip[i+1] = args[i+1], true, true, true
		} else if rest, ok := strings.CutPrefix(a, "--run="); ok {
			runName, runSet, skip[i] = rest, true, true
		} else if a == "--tar" {
			tarSet, skip[i] = true, true
		}
	}
	for i := range args {
		if !skip[i] {
			pos = append(pos, args[i])
		}
	}
	return pos, cfgPath, runName, cfgSet, runSet, tarSet
}

// Выбирает конфиг: --config файл;
// иначе default.yaml рядом;
// иначе спрашивает, использовать ли встроенный.
func resolveConfig(lg interface{ Printf(string, ...any) }, cfgPath string, cfgSet bool) error {
	if cfgSet {
		lg.Printf("CONFIG file %q", cfgPath)
		return config.LoadFile(cfgPath)
	}
	const local = "default.yaml"
	if _, err := os.Stat(local); err == nil {
		lg.Printf("CONFIG file %q (found nearby)", local)
		return config.LoadFile(local)
	}
	fmt.Fprint(os.Stderr, "Warning! No config found. Continue with default config? (y/n) ")
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		return fmt.Errorf("no config: no --config, no ./default.yaml (no input, fail-closed)")
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	if ans != "y" && ans != "yes" {
		return fmt.Errorf("no config: no --config, no ./default.yaml (declined)")
	}
	lg.Printf("CONFIG built-in default (confirmed, nothing written)")
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]

	if cmd == "init-config" {
		path := "default.yaml"
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		if err := config.WriteDefault(path); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
		return
	}

	logger, closeLog, err := logger.Init(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer closeLog()
	logger.Log("START %s", strings.Join(os.Args[1:], " "))
	pos, cfgPath, runName, cfgSet, runSet, tarSet := splitArgs(os.Args[2:])

	if cmd != "pack" && cmd != "unpack" {
		usage()
		os.Exit(1)
	}
	if len(pos) != 2 {
		usage()
		os.Exit(1)
	}
	if err := resolveConfig(logger, cfgPath, cfgSet); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	if cmd == "unpack" {
		src := pos[0]
		if strings.EqualFold(filepath.Ext(src), ".gz") {
			stage, err := unpack.UnpackTar(src)
			if err != nil {
				logger.Error("%v", err)
				os.Exit(1)
			}
			defer os.RemoveAll(stage)
			src = stage
		}
		count, err := unpack.UnpackDir(src, pos[1])
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		logger.Result("unpack %s -> %s files=%d", pos[0], pos[1], count)
		return
	}
	if !runSet || runName == "" {
		runName = "QPack_" + filepath.Base(filepath.Clean(pos[0]))
	}
	if tarSet {
		arcPath := filepath.Join(pos[1], runName+".tar.gz")
		count, err := pack.PackDirTo(pos[0], arcPath, runName)
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		logger.Result("pack %s -> %s files=%d (archive)", pos[0], arcPath, count)
		return
	}
	runDir, count, err := pack.ConvertDir(pos[0], pos[1], runName)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	logger.Result("pack %s -> %s files=%d", pos[0], runDir, count)
}
