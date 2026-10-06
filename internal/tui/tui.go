package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qpack/internal/config"
	"qpack/internal/logger"
	"qpack/internal/pack"
	"qpack/internal/unpack"
)

// ResolveConfig — выбор конфига: --config файл; иначе default.yaml рядом;
// иначе спрашивает, использовать ли встроенный (отказ/EOF = fail-closed).
func ResolveConfig(lg interface{ Printf(string, ...any) }, cfgPath string, cfgSet bool) error {
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
	ans, ok := prompt("")
	if !ok {
		return fmt.Errorf("no config: no --config, no ./default.yaml (no input, fail-closed)")
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	if ans != "y" && ans != "yes" {
		return fmt.Errorf("no config: no --config, no ./default.yaml (declined)")
	}
	lg.Printf("CONFIG built-in default (confirmed, nothing written)")
	return nil
}

var stdinReader = bufio.NewReader(os.Stdin)

func prompt(label string) (string, bool) {
	fmt.Fprint(os.Stdout, label)
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(line), true
}

// Run — интерактивное меню в терминале. Возвращает код выхода.
func Run() int {
	for {
		fmt.Fprintln(os.Stdout, "qpack — choose action:")
		fmt.Fprintln(os.Stdout, "  1 pack .dat -> .qpac")
		fmt.Fprintln(os.Stdout, "  2 unpack .qpac -> .dat")
		fmt.Fprintln(os.Stdout, "  3 init-config")
		fmt.Fprintln(os.Stdout, "  q quit")
		choice, ok := prompt("> ")
		if !ok {
			return 0
		}
		switch strings.ToLower(choice) {
		case "1":
			doPack()
		case "2":
			doUnpack()
		case "3":
			doInitConfig()
		case "q", "quit", "exit":
			pauseIfTTY()
			return 0
		default:
			fmt.Fprintln(os.Stdout, "unknown choice, try 1/2/3/q")
		}
	}
}

func withSpinner(fn func() string) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(300 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				fmt.Fprint(os.Stderr, ".")
			}
		}
	}()
	msg := fn()
	close(done)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stdout, msg)
}

func cfgFromAnswer(lg interface{ Printf(string, ...any) }, ans string) error {
	if ans != "" {
		return ResolveConfig(lg, ans, true)
	}
	return ResolveConfig(lg, "", false)
}

func doPack() {
	inDir, ok := prompt("input dir (.dat): ")
	if !ok || inDir == "" {
		return
	}
	outDir, ok := prompt("output dir: ")
	if !ok || outDir == "" {
		return
	}
	runName, ok := prompt("run name (empty = auto): ")
	if !ok {
		return
	}
	cfgPath, ok := prompt("config path (empty = auto): ")
	if !ok {
		return
	}
	tarAns, ok := prompt("tar.gz archive? (y/n): ")
	if !ok {
		return
	}
	if !runSet(runName) {
		runName = "QPack_" + filepath.Base(filepath.Clean(inDir))
	}
	lg, closeLog, err := logger.Init("pack")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	defer closeLog()
	lg.Log("START pack %s %s", inDir, outDir)
	if err := cfgFromAnswer(lg, cfgPath); err != nil {
		lg.Error("%v", err)
		return
	}
	tar := strings.ToLower(strings.TrimSpace(tarAns)) == "y" || strings.ToLower(strings.TrimSpace(tarAns)) == "yes"
	withSpinner(func() string {
		if tar {
			arcPath := filepath.Join(outDir, runName+".tar.gz")
			count, err := pack.PackDirTo(inDir, arcPath, runName)
			if err != nil {
				lg.Error("%v", err)
				return fmt.Sprintf("error: %v", err)
			}
			msg := fmt.Sprintf("pack %s -> %s files=%d (archive)", inDir, arcPath, count)
			lg.Result("%s", msg)
			return msg
		}
		runDir, count, err := pack.ConvertDir(inDir, outDir, runName)
		if err != nil {
			lg.Error("%v", err)
			return fmt.Sprintf("error: %v", err)
		}
		msg := fmt.Sprintf("pack %s -> %s files=%d", inDir, runDir, count)
		lg.Result("%s", msg)
		return msg
	})
}

func doUnpack() {
	src, ok := prompt("input (.qpac dir or .tar.gz): ")
	if !ok || src == "" {
		return
	}
	outBase, ok := prompt("output dir: ")
	if !ok || outBase == "" {
		return
	}
	runName, ok := prompt("run name (empty = auto): ")
	if !ok {
		return
	}
	cfgPath, ok := prompt("config path (empty = auto): ")
	if !ok {
		return
	}
	lg, closeLog, err := logger.Init("unpack")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	defer closeLog()
	lg.Log("START unpack %s %s", src, outBase)
	if err := cfgFromAnswer(lg, cfgPath); err != nil {
		lg.Error("%v", err)
		return
	}
	if !runSet(runName) {
		runName = "QUnpack_" + filepath.Base(filepath.Clean(src))
	}
	outDir := filepath.Join(outBase, runName)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		lg.Error("mkdir out dir %q: %v", outDir, err)
		return
	}
	if strings.EqualFold(filepath.Ext(src), ".gz") {
		stage, err := unpack.UnpackTar(src)
		if err != nil {
			lg.Error("%v", err)
			return
		}
		defer os.RemoveAll(stage)
		src = stage
	}
	withSpinner(func() string {
		count, err := unpack.UnpackDir(src, outDir)
		if err != nil {
			lg.Error("%v", err)
			return fmt.Sprintf("error: %v", err)
		}
		msg := fmt.Sprintf("unpack -> %s files=%d", outDir, count)
		lg.Result("%s", msg)
		return msg
	})
}

func doInitConfig() {
	path, ok := prompt("config path [default.yaml]: ")
	if !ok {
		return
	}
	if path == "" {
		path = "default.yaml"
	}
	if err := config.WriteDefault(path); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	fmt.Fprintln(os.Stdout, "wrote", path)
}

func runSet(s string) bool { return strings.TrimSpace(s) != "" }

func pauseIfTTY() {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return
	}
	fmt.Fprintln(os.Stdout, "Press Enter to exit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
