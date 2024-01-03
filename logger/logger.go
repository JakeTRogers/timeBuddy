package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var log zerolog.Logger

func init() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339

	var output io.Writer = zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}

	log = zerolog.New(output).
		Level(zerolog.ErrorLevel).
		With().
		Timestamp().
		Logger()
}

func GetLogger() *zerolog.Logger {
	return &log
}

func SetLogLevel(verboseCount int) {
	var level zerolog.Level
	switch {
	case verboseCount == 1:
		level = (zerolog.WarnLevel)
	case verboseCount == 2:
		level = (zerolog.InfoLevel)
	case verboseCount == 3:
		level = (zerolog.DebugLevel)
	case verboseCount >= 4:
		level = (zerolog.TraceLevel)
	default:
		level = (zerolog.ErrorLevel)
	}
	log = log.Level(level)
}
