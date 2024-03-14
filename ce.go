package ce

import (
	"context"
	_ "expvar"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type slogWrapper struct {
	*slog.Logger
}

var DefaultLevel slog.LevelVar

var defaultLogger atomic.Value

func Default() *slogWrapper { return defaultLogger.Load().(*slogWrapper) }

func SetDefault(l *slog.Logger) { defaultLogger.Store(&slogWrapper{l}) }

func init() {
	DefaultLevel.Set(slog.LevelDebug)

	log := slog.New(
		slog.NewJSONHandler(
			os.Stderr,
			&slog.HandlerOptions{
				AddSource: true,
				Level:     &DefaultLevel,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.SourceKey {
						source := a.Value.Any().(*slog.Source)
						source.File = filepath.Base(source.File)
					}
					return a
				},
			},
		),
	)

	defaultLogger.Store(&slogWrapper{log})
}

func (l *slogWrapper) log(ctx context.Context, level slog.Level, skip int, msg string, args ...any) {
	if !l.Enabled(ctx, level) {
		return
	}

	var pc uintptr
	{
		if skip < 0 {
			skip = 3
		}

		var pcs [1]uintptr
		// skip [runtime.Callers, this function, this function's caller]
		runtime.Callers(skip, pcs[:])
		pc = pcs[0]
	}
	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.Add(args...)
	if ctx == nil {
		ctx = context.Background()
	}
	_ = l.Handler().Handle(ctx, r)
}

func Print(args ...any) {
	var argsNew []any
	for k, v := range args {
		argsNew = append(argsNew, strconv.Itoa(k))
		argsNew = append(argsNew, v)
	}
	Default().log(nil, slog.LevelDebug, -1, "Print", argsNew...)
}

func Printf(format string, args ...any) {
	Default().log(nil, slog.LevelDebug, -1, "Printf", slog.String("0", fmt.Sprintf(format, args...)))
}

func Debug(msg string, args ...any) {
	Default().log(nil, slog.LevelDebug, -1, msg, args...)
}

func Info(msg string, args ...any) {
	Default().log(nil, slog.LevelInfo, -1, msg, args...)
}

func Warn(msg string, args ...any) {
	Default().log(nil, slog.LevelWarn, -1, msg, args...)
}

func Error(msg string, args ...any) {
	Default().log(nil, slog.LevelError, -1, msg, args...)
}

type panicByCheckError struct {
	OriginalErr error
}

func (p *panicByCheckError) Error() string {
	return p.OriginalErr.Error()
}

func CheckError(err error, args ...any) {
	if err != nil {
		l := make([]any, 0, len(args)+2)
		l = append(l, "err", err)
		l = append(l, args...)
		Default().log(nil, slog.LevelError, -1, "checkError", l...)
		panic(&panicByCheckError{OriginalErr: err})
	}
}

var Recover = func(showStack bool, defers ...func(recoverObj interface{})) {
	if recoverObj := recover(); recoverObj != nil {
		_, ok := recoverObj.(*panicByCheckError)
		if ok == false {
			if showStack {
				callStackBin := debug.Stack()
				Default().log(nil, slog.LevelError, 4, "recover", "recoverObj", recoverObj, "callStack", *(*string)(unsafe.Pointer(&callStackBin)))
			} else {
				Default().log(nil, slog.LevelError, 4, "recover", "recoverObj", recoverObj)
			}
		}

		for _, v := range defers {
			v(recoverObj)
		}
	}
}

var Notify = func(callbacks ...func()) {
	notifyObjChan := make(chan os.Signal, 1)
	signal.Notify(notifyObjChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)

forByNotify:
	for {
		s := <-notifyObjChan
		Info("notify", slog.Any("notifyObj", s))
		switch s {
		case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM:
			break forByNotify

		case syscall.SIGHUP:
		default:
			break forByNotify
		}
	}

	for _, callback := range callbacks {
		callback()
	}
}

func OpenPProf(addr string) {
	go func() {
		defer Recover(true)

		err := http.ListenAndServe(addr, nil)
		CheckError(err)
	}()
}

type errWrapper struct {
	Origin interface{}
	err    error
}

func (w *errWrapper) Error() string {
	return w.err.Error()
}

func WrapToErr(obj any) error {
	return &errWrapper{
		Origin: obj,
		err:    fmt.Errorf("%w", obj),
	}
}
