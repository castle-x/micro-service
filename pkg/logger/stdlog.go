package logger

import (
	"log"
	"strings"

	"go.uber.org/zap"
)

const stdLogCallerSkip = 3

// IngestStdLog redirects the standard library default logger into the global
// zap logger. The returned function restores the previous stdlib logger state.
func IngestStdLog() func() {
	prevWriter := log.Writer()
	prevFlags := log.Flags()
	prevPrefix := log.Prefix()

	log.SetOutput(stdLogWriter{})
	log.SetFlags(0)
	log.SetPrefix("")

	return func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
	}
}

type stdLogWriter struct{}

func (stdLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}
	if lg := L(); lg != nil {
		lg.WithOptions(zap.AddCallerSkip(stdLogCallerSkip)).Info(msg)
	}
	return len(p), nil
}
