package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Log levels
const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var (
	mu       sync.Mutex
	level    = LevelDebug // default
	debugLog = log.New(os.Stdout, "DEBUG: ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
	infoLog  = log.New(os.Stdout, "INFO:  ", log.LstdFlags|log.Lmicroseconds)
	warnLog  = log.New(os.Stdout, "WARN:  ", log.LstdFlags|log.Lmicroseconds)
	errorLog = log.New(os.Stderr, "ERROR: ", log.LstdFlags|log.Lmicroseconds)
	fatalLog = log.New(os.Stderr, "FATAL: ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
)

func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	debugLog.SetOutput(w)
	infoLog.SetOutput(w)
	warnLog.SetOutput(w)
	errorLog.SetOutput(w)
	fatalLog.SetOutput(w)
}

func SetLevel(l int) {
	mu.Lock()
	defer mu.Unlock()
	level = l
}

func Debug(format string, v ...any) {
	if level <= LevelDebug {
		debugLog.Output(2, sprint(format, v...))
	}
}

func Info(format string, v ...any) {
	if level <= LevelInfo {
		infoLog.Output(2, sprint(format, v...))
	}
}

func Warn(format string, v ...any) {
	if level <= LevelWarn {
		warnLog.Output(2, sprint(format, v...))
	}
}

func Error(format string, v ...any) {
	if level <= LevelError {
		errorLog.Output(2, sprint(format, v...))
	}
}

func Fatal(format string, v ...any) {
	if level <= LevelFatal {
		fatalLog.Output(2, sprint(format, v...))
	}
}

func sprint(format string, v ...any) string {
	return fmt.Sprintf(format, v...)
}
