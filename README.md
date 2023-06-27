# actionslog

[![Go Reference](https://pkg.go.dev/badge/github.com/willabides/actionslog.svg)](https://pkg.go.dev/github.com/willabides/actionslog)

```shell
go get github.com/willabides/actionslog
```

actionslog provides a handler for Go's [log/slog](https://pkg.go.dev/log/slog) that outputs logs in the format expected
by GitHub Actions.

The output format is roughly `<message>\n<vars as yaml>` after GitHub is done with it. So, in your output,
`logger.Info("hello", slog.String("object", "world"), slog.Any("list", []string{"a", "b", "c"))` would look like:

```
hello
object: world
list:
  - a
  - b  
```

actionslog.Handler implements [slog.Handler](https://pkg.go.dev/log/slog#Handler) from "log/slog" when built with go
1.21 or higher. When built with go earlier versions that predate "log/slog", it
implements [slog.Handler](https://pkg.go.dev/golang.org/x/exp/slog#Handler) from "golang.org/x/exp/slog".

## Usage

```go
package main

import (
	"log/slog"
	"os"

	"github.com/willabides/actionslog"
)

func main() {
	logger := slog.New(actionslog.New(
		os.Stdout,
		&actionslog.Options{
			AddSource: true,
			Level:     slog.LevelDebug,
		},
	))

	logger.Info("hello", slog.String("object", "world"))
}
```

## Screenshots

This is what it looks like on the GitHub UI:

### Run Log

![run log](./doc/example_log.png)

### Workflow Summary

![workflow summary](./doc/example_summary.png)

### Inline Code Annotations

![inline code annotations](./doc/example_inline.png)
