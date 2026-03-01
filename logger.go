package main

import (
	"fmt"
	"os"
	"time"
)

const logTimeFormat = "2006-01-02 15:04:05"

func logInfof(format string, args ...any) {
	logf(os.Stdout, "INFO", format, args...)
}

func logWarnf(format string, args ...any) {
	logf(os.Stderr, "WARN", format, args...)
}

func logErrorf(format string, args ...any) {
	logf(os.Stderr, "ERROR", format, args...)
}

func logf(out *os.File, level string, format string, args ...any) {
	timestamp := time.Now().Format(logTimeFormat)
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(out, "%s [%s] %s\n", timestamp, level, message)
}
