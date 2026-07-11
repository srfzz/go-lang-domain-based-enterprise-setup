package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// dailyFileWriter rotates log files at midnight based on the current date.
type dailyFileWriter struct {
	dir      string
	baseName string
	ext      string
	file     *os.File
	mu       sync.Mutex
}

func newDailyFileWriter(logPath string) *dailyFileWriter {
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return &dailyFileWriter{dir: dir, baseName: name, ext: ext}
}

func (w *dailyFileWriter) currentDate() string {
	return time.Now().Format("2006-01-02")
}

func (w *dailyFileWriter) dailyPath() string {
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s%s", w.baseName, w.currentDate(), w.ext))
}

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	path := w.dailyPath()
	if w.file == nil {
		os.MkdirAll(w.dir, 0755)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		w.file = f
	}
	// If the path changed (new day), close old and open new
	if w.file.Name() != path {
		w.file.Close()
		os.MkdirAll(w.dir, 0755)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		w.file = f
	}
	return w.file.Write(p)
}

func (w *dailyFileWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

func Init(level string, filePath string) {
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	cores := []zapcore.Core{
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), parseLevel(level)),
	}
	if filePath != "" {
		writer := newDailyFileWriter(filePath)
		fileEncoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(writer), parseLevel(level)))
	}
	log = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	zap.ReplaceGlobals(log)
}

func parseLevel(lvl string) zapcore.Level {
	switch lvl {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func Sync()                                 { _ = log.Sync() }
func Info(msg string, fields ...zap.Field)  { log.Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { log.Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { log.Fatal(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { log.Debug(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { log.Warn(msg, fields...) }
