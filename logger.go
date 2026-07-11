package contexting

import (
	"fmt"
	"os"
	"time"
)

const logTimeFormat = "2006-01-02 15:04:05"

func LogInfof(format string, args ...any) {
	if logToStderr {
		logf(os.Stderr, "INFO", format, args...)
	} else {
		logf(os.Stdout, "INFO", format, args...)
	}
}

func LogWarnf(format string, args ...any) {
	logf(os.Stderr, "WARN", format, args...)
}

func LogErrorf(format string, args ...any) {
	logf(os.Stderr, "ERROR", format, args...)
}

func logf(out *os.File, level string, format string, args ...any) {
	timestamp := time.Now().Format(logTimeFormat)
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(out, "%s [%s] %s\n", timestamp, level, message)
}
