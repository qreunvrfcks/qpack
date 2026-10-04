package logger

// Формат строк в файле: START / DONE / ERROR / SKIP.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu     sync.Mutex
	std    = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	active = std
)

type Logger struct {
	file *log.Logger
	out  *log.Logger
}

// Создает log/<tool>.log. Возвращает логгер и Close.
func Init(tool string) (*Logger, func(), error) {
	if err := os.MkdirAll("log", 0755); err != nil {
		return nil, nil, fmt.Errorf("mkdir log: %w", err)
	}
	f, err := os.OpenFile(filepath.Join("log", tool+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	lg := &Logger{
		file: log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		out:  log.New(os.Stderr, "", 0),
	}
	mu.Lock()
	active = lg.file
	mu.Unlock()
	return lg, func() { _ = f.Close() }, nil
}

// Пишет через активный логгер.
func Printf(format string, v ...any) {
	mu.Lock()
	l := active
	mu.Unlock()
	l.Printf(format, v...)
}

// Пишет ошибку в .log файл (с префиксом ERROR) и в консоль.
func Error(format string, v ...any) {
	mu.Lock()
	l := active
	mu.Unlock()
	msg := fmt.Sprintf(format, v...)
	l.Printf("ERROR %s", msg)
	log.New(os.Stderr, "", 0).Printf("error: %s", msg)
}

// Пишет в .log файл (START, SKIP, CONFIG).
func (l *Logger) Log(format string, v ...any) { l.file.Printf(format, v...) }

func (l *Logger) Printf(format string, v ...any) { l.Log(format, v...) }

// Пишет ошибку в файл и в консоль.
func (l *Logger) Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.file.Printf("ERROR %s", msg)
	l.out.Printf("error: %s", msg)
}

// Пишет финальный результат в .log файл (DONE) и в консоль.
func (l *Logger) Result(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.file.Printf("DONE %s", msg)
	l.out.Println(msg)
}
