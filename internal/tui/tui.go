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

// pickDir — выбор папки: графический диалог (проводник), иначе ввод текстом.
// Пустой ответ в диалоге = отмена, тогда тоже падаем на ручной ввод.
func pickDir(label string) (string, bool) {
	if dir, ok := guiPickDir(label); ok && dir != "" {
		fmt.Fprintln(os.Stdout, label+dir)
		return dir, true
	}
	fmt.Fprintln(os.Stdout, "(проводник недоступен — введи путь текстом)")
	for {
		dir, ok := prompt(label)
		if !ok {
			return "", false
		}
		if dir == "" {
			return "", true
		}
		if st, err := os.Stat(dir); err == nil {
			if st.IsDir() {
				return dir, true
			}
			fmt.Fprintln(os.Stdout, "это файл, нужна папка (пусто = назад)")
			continue
		}
		fmt.Fprintf(os.Stdout, "папки нет, создать %q? (y/n): ", dir)
		ans, ok := prompt("")
		if !ok {
			return "", false
		}
		if strings.ToLower(strings.TrimSpace(ans)) == "y" || strings.ToLower(strings.TrimSpace(ans)) == "yes" {
			return dir, true
		}
	}
}

// pickUnpackSrc — вход unpack: папка .qpac или архив .tar.gz.
// Тип определяет сама прога по расширению (.gz = архив).
func pickUnpackSrc(label string) (string, bool) {
	kind, ok := prompt("input: [1] .qpac dir  [2] archive (.tar.gz/.zip): ")
	if !ok {
		return "", false
	}
	if strings.TrimSpace(kind) == "2" {
		return pickArchive(label)
	}
	return pickQpacDir(label)
}

func pickQpacDir(label string) (string, bool) {
	if dir, ok := guiPickDir(label); ok && dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			fmt.Fprintln(os.Stdout, label+dir)
			return dir, true
		}
	}
	fmt.Fprintln(os.Stdout, "(проводник недоступен — введи путь текстом)")
	for {
		dir, ok := prompt(label)
		if !ok {
			return "", false
		}
		if dir == "" {
			return "", true
		}
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir, true
		}
		fmt.Fprintln(os.Stdout, "нужна папка .qpac, попробуй ещё (пусто = назад)")
	}
}

func pickArchive(label string) (string, bool) {
	if src, ok := guiPickArchive(label); ok && validArchive(src) {
		fmt.Fprintln(os.Stdout, label+src)
		return src, true
	}
	fmt.Fprintln(os.Stdout, "(проводник недоступен — введи путь текстом)")
	for {
		src, ok := prompt(label)
		if !ok {
			return "", false
		}
		if src == "" {
			return "", true
		}
		if validArchive(src) {
			return src, true
		}
		fmt.Fprintln(os.Stdout, "нужен .tar.gz или .zip, попробуй ещё (пусто = назад)")
	}
}

func validArchive(src string) bool {
	if src == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(src))
	if ext != ".gz" && ext != ".zip" {
		return false
	}
	st, err := os.Stat(src)
	return err == nil && !st.IsDir()
}

func validUnpackSrc(src string) bool {
	if src == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".gz" || ext == ".zip" {
		st, err := os.Stat(src)
		return err == nil && !st.IsDir()
	}
	st, err := os.Stat(src)
	return err == nil && st.IsDir()
}

// Run — интерактивное меню в терминале. Возвращает код выхода.
func Run() int {
	for {
		fmt.Fprintln(os.Stdout, "qpack — choose action:")
		fmt.Fprintln(os.Stdout, "  [pack] .dat -> .qpac")
		fmt.Fprintln(os.Stdout, "  [unpack] .qpac -> .dat")
		fmt.Fprintln(os.Stdout, "  [config] set config")
		fmt.Fprintln(os.Stdout, "  [q] quit")
		choice, ok := prompt("> ")
		if !ok {
			return 0
		}
		switch strings.ToLower(choice) {
		case "pack":
			doPack()
		case "unpack":
			doUnpack()
		case "config":
			doInitConfig()
		case "q", "quit", "exit":
			pauseIfTTY()
			return 0
		default:
			fmt.Fprintln(os.Stdout, "unknown choice, try pack/unpack/config/q")
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

// askConfig — один вопрос: [1] auto  [2] import config (диалог файла).
// Возвращает cfgPath/cfgSet для ResolveConfig; ok=false = назад.
func askConfig() (cfgPath string, cfgSet, ok bool) {
	ans, ok := prompt("config: [1] auto  [2] import file: ")
	if !ok {
		return "", false, false
	}
	if strings.TrimSpace(ans) != "2" {
		return "", false, true
	}
	if path, ok := guiPickFile("Import config", "*.yaml *.yml|YAML configs"); ok && path != "" {
		fmt.Fprintln(os.Stdout, "config: "+path)
		return path, true, true
	}
	fmt.Fprintln(os.Stdout, "(проводник недоступен — введи путь текстом)")
	path, ok := prompt("config file (empty = auto): ")
	if !ok {
		return "", false, false
	}
	if strings.TrimSpace(path) == "" {
		return "", false, true
	}
	return strings.TrimSpace(path), true, true
}

func doPack() {
	inDir, ok := pickDir("input dir (.dat): ")
	if !ok || inDir == "" {
		return
	}
	outDir, ok := pickDir("output dir: ")
	if !ok || outDir == "" {
		return
	}
	runName, ok := prompt("run name (empty = auto): ")
	if !ok {
		return
	}
	cfgPath, cfgSet, ok := askConfig()
	if !ok {
		return
	}
	arcAns, ok := prompt("archive? [1] none  [2] tar.gz  [3] zip: ")
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
	if err := ResolveConfig(lg, cfgPath, cfgSet); err != nil {
		lg.Error("%v", err)
		return
	}
	arc := strings.TrimSpace(arcAns)
	withSpinner(func() string {
		if arc == "2" {
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
		if arc == "3" {
			arcPath := filepath.Join(outDir, runName+".zip")
			count, err := pack.PackDirToZip(inDir, arcPath, runName)
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
	src, ok := pickUnpackSrc("input: ")
	if !ok || src == "" {
		return
	}
	outBase, ok := pickDir("output dir: ")
	if !ok || outBase == "" {
		return
	}
	runName, ok := prompt("run name (empty = auto): ")
	if !ok {
		return
	}
	cfgPath, cfgSet, ok := askConfig()
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
	if err := ResolveConfig(lg, cfgPath, cfgSet); err != nil {
		lg.Error("%v", err)
		return
	}
	if !runSet(runName) {
		base := filepath.Base(filepath.Clean(src))
		base = strings.TrimSuffix(base, filepath.Ext(base))
		base = strings.TrimSuffix(base, ".tar")
		base = strings.TrimSuffix(base, ".zip")
		runName = "QUnpack_" + base
	}
	outDir := filepath.Join(outBase, runName)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		lg.Error("mkdir out dir %q: %v", outDir, err)
		return
	}
	if strings.EqualFold(filepath.Ext(src), ".zip") {
		stage, content, err := unpack.UnpackZip(src)
		if err != nil {
			lg.Error("%v", err)
			return
		}
		defer os.RemoveAll(stage)
		src = content
	} else if strings.EqualFold(filepath.Ext(src), ".gz") {
		stage, content, err := unpack.UnpackTar(src)
		if err != nil {
			lg.Error("%v", err)
			return
		}
		defer os.RemoveAll(stage)
		src = content
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
