package logger

import (
	"github.com/itsLeonB/ezutil/v2"
	"github.com/itsLeonB/ezutil/v2/otel"
)

var Global ezutil.Logger

func Init(appNamespace string) {
	Global = otel.Init(appNamespace)
}

func Debug(args ...any) {
	Global.Debug(args...)
}

func Info(args ...any) {
	Global.Info(args...)
}

func Warn(args ...any) {
	Global.Warn(args...)
}

func Error(args ...any) {
	Global.Error(args...)
}

func Fatal(args ...any) {
	Global.Fatal(args...)
}

func Debugf(format string, args ...any) {
	Global.Debugf(format, args...)
}

func Infof(format string, args ...any) {
	Global.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	Global.Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	Global.Errorf(format, args...)
}

func Fatalf(format string, args ...any) {
	Global.Fatalf(format, args...)
}
