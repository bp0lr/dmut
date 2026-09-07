package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
)

// Set by release builds with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() { os.Exit(exitCode()) }

func exitCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = os.Stdin.Close()
	}()
	if err := runCLI(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "dmut:", err)
		if ctx.Err() != nil {
			return 130
		}
		var usage *usageError
		if errors.As(err, &usage) {
			return 2
		}
		return 1
	}
	return 0
}

func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func runCLI(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, done, err := parseConfig(args, stdout, stderr)
	if err != nil || done {
		return err
	}
	return newApplication(cfg, stdout, stderr).run(ctx, stdin)
}
