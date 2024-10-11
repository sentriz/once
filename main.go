package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gofrs/flock"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) <= 1 {
		return fmt.Errorf("need <cmd> [<args>...]")
	}
	cmd, args := os.Args[1], os.Args[2:]

	cacheHome, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("find user cache dir: %w", err)
	}

	lockPath := filepath.Join(cacheHome, "once-lock")
	pidPath := filepath.Join(cacheHome, "once-pid")

	fl := flock.New(lockPath)
	defer fl.Close()

	if err := fl.Lock(); err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	otherPid, err := os.ReadFile(pidPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read other: %w", err)
	}
	if oth := strings.TrimSpace(string(otherPid)); oth != "" {
		if err := exec.Command("kill", oth).Run(); err != nil {
			if exitErr := (&exec.ExitError{}); !errors.As(err, &exitErr) {
				return fmt.Errorf("exec kill: %w", err)
			}
		}
	}

	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}

	if err := fl.Unlock(); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		return syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
	}

	if err := c.Run(); err != nil {
		return fmt.Errorf("run command: %w", err)
	}
	return nil
}
