package logging

import (
	"log/slog"
	"os"

	"github.com/mobilefarm/af/scenarios/internal/port"
)

type Slog struct {
	l *slog.Logger
}

func New(level slog.Level) *Slog {
	return &Slog{
		l: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})),
	}
}

func (s *Slog) With(args ...any) *Slog {
	return &Slog{l: s.l.With(args...)}
}

func (s *Slog) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s *Slog) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s *Slog) Error(msg string, args ...any) { s.l.Error(msg, args...) }
func (s *Slog) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }

var _ port.Logger = (*Slog)(nil)
