package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger - обёртка над zap.Logger с дополнительными методами
type Logger struct {
	*zap.Logger
}

// Options опции для создания логгера
type Options struct {
	Level  string
	Format string
	Async  bool // Асинхронное логирование с буферизацией
}

// New создаёт новый логгер
func New(level string, format string) (*Logger, error) {
	return NewWithOptions(Options{
		Level:  level,
		Format: format,
		Async:  false,
	})
}

// NewWithOptions создаёт новый логгер с заданными опциями
func NewWithOptions(opts Options) (*Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	var encoderConfig zapcore.EncoderConfig

	if opts.Format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if opts.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Создаём WriteSyncer
	var writeSyncer zapcore.WriteSyncer
	if opts.Async {
		// Асинхронное логирование с буферизацией (8KB буфер, flush каждые 30 секунд)
		writeSyncer = &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stdout),
			Size:          8 * 1024, // 8KB буфер
			FlushInterval: 0,        // Сброс только при заполнении буфера или Sync()
		}
	} else {
		// Синхронное логирование
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{Logger: logger}, nil
}

// LogError логирует ошибку с контекстом
func (l *Logger) LogError(msg string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	l.Error(msg, fields...)
}
