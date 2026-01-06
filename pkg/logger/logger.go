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

// NewAsync создаёт новый логгер с асинхронным логированием
func NewAsync(level string, format string) (*Logger, error) {
	return NewWithOptions(Options{
		Level:  level,
		Format: format,
		Async:  true,
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
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Создаём WriteSyncer
	var writeSyncer zapcore.WriteSyncer
	if opts.Async {
		// Асинхронное логирование с буферизацией (8KB buffer, flush каждые 30 секунд)
		writeSyncer = &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stdout),
			Size:          8 * 1024, // 8KB buffer
			FlushInterval: 0,        // Flush только при заполнении буфера или Sync()
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

// WithFields добавляет дополнительные поля к логгеру
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...)}
}

// WithRequestID добавляет request_id к логгеру
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithFields(zap.String("request_id", requestID))
}

// WithUserID добавляет user_id к логгеру
func (l *Logger) WithUserID(userID string) *Logger {
	return l.WithFields(zap.String("user_id", userID))
}

// WithMatchID добавляет match_id к логгеру
func (l *Logger) WithMatchID(matchID string) *Logger {
	return l.WithFields(zap.String("match_id", matchID))
}

// WithTournamentID добавляет tournament_id к логгеру
func (l *Logger) WithTournamentID(tournamentID string) *Logger {
	return l.WithFields(zap.String("tournament_id", tournamentID))
}

// LogError логирует ошибку с контекстом
func (l *Logger) LogError(msg string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	l.Error(msg, fields...)
}

// Sync синхронизирует буфер логгера
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}
