# ce
Simple wrapper for standard library log/slog

### usage

```
package main

import (
	"github.com/themoment198/ce"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	log := slog.New(
		slog.NewJSONHandler(
			io.MultiWriter(
				os.Stderr,
				&lumberjack.Logger{
					Filename:   "/tmp/foobar.txt",
					MaxSize:    1,  // mb
					MaxAge:     30, // days
					MaxBackups: 7,  // count of log file
					LocalTime:  false,
					Compress:   false,
				},
			),

			&slog.HandlerOptions{
				AddSource: true,
				Level:     &ce.DefaultLevel,
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

	ce.SetDefault(log)

	// case 1
	func() {
		ce.Debug("test1", slog.String("k1", "v1"), slog.Int("k2", 2))
		ce.Info("test2")
		ce.Warn("test3")
		ce.Error("test4")
		ce.DefaultLevel.Set(slog.LevelInfo)
		ce.Debug("test5")
	}()

	// case 2
	func() {
		defer ce.Recover(false)
		ce.CheckError(io.EOF, slog.String("k1", "v1"))
	}()

	// case 3
	func() {
		defer ce.Recover(false)
		panic(io.EOF)
	}()
}


```
