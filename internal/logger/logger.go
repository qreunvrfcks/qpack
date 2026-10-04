package logger

// Логирование CLI: всё пишется в файл log/<tool>.log;
// в консоль (stderr) идут только ошибки и финальный результат.
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

// Logger — файловый логгер плюс консольный вывод ошибок/результата.
type Logger struct {
	file *log.Logger // всё подряд
	out  *log.Logger // только консоль (ошибки и результат)
}

// Init открывает (создаёт) log/<tool>.log. Возвращает логгер и Close для defer.
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

// Printf пишет через активный логгер (после Init — только в файл,
// до Init — в stderr). Для библиотечного кода (скипы convertor).
func Printf(format string, v ...any) {
	mu.Lock()
	l := active
	mu.Unlock()
	l.Printf(format, v...)
}

// Error пишет ошибку в файл (с префиксом ERROR) и в консоль.
// Для библиотечного кода (битые файлы convertor).
func Error(format string, v ...any) {
	mu.Lock()
	l := active
	mu.Unlock()
	msg := fmt.Sprintf(format, v...)
	l.Printf("ERROR %s", msg)
	log.New(os.Stderr, "", 0).Printf("error: %s", msg)
}

// Log — только в файл (обычные события: START, SKIP, CONFIG).
func (l *Logger) Log(format string, v ...any) { l.file.Printf(format, v...) }

// Printf — для совместимости: только в файл (см. Log).
func (l *Logger) Printf(format string, v ...any) { l.Log(format, v...) }

// Error — в файл и в консоль.
func (l *Logger) Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.file.Printf("ERROR %s", msg)
	l.out.Printf("error: %s", msg)
}

// Result — финальный результат: в файл как DONE и в консоль чистым текстом.
func (l *Logger) Result(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.file.Printf("DONE %s", msg)
	l.out.Println(msg)
}
