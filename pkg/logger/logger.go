package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger обёртка над zap, добавляет только LogError
type Logger struct {
	*zap.Logger
}

type Options struct {
	Level  string
	Format string
	Async  bool // буферизованный вывод
}

func New(level string, format string) (*Logger, error) {
	return NewWithOptions(Options{
		Level:  level,
		Format: format,
		Async:  false,
	})
}

func NewWithOptions(opts Options) (*Logger, error) {
	var zapLevel zapcore.Level
	// кривой уровень в конфиге не роняем сервис, просто откатываемся в info
	if err := zapLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	var encoderConfig zapcore.EncoderConfig
	if opts.Format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}
	// ts + iso8601 - под них настроен парсинг в loki, не менять
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if opts.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var writeSyncer zapcore.WriteSyncer
	if opts.Async {
		// буфер 8кб, флашится когда заполнится или по Sync() (mains делают defer Sync)
		writeSyncer = &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stdout),
			Size:          8 * 1024,
			FlushInterval: 0,
		}
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	// CallerSkip(1) - вызовы идут через методы обёртки, иначе caller будет врать
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{Logger: logger}, nil
}

// LogError логирует ошибку, добавляя её в поле error
func (l *Logger) LogError(msg string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	l.Error(msg, fields...)
}
