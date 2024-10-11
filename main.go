package main

import (
	"context"
	"errors"
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
	if len(os.Args) <= 1 {
		os.Exit(1)
	}
	cmd, args := os.Args[1], os.Args[2:]

	cacheHome, err := os.UserCacheDir()
	if err != nil {
		os.Exit(1)
	}

	lockPath := filepath.Join(cacheHome, "once-lock")
	pidPath := filepath.Join(cacheHome, "once-pid")

	fl := flock.New(lockPath)
	defer fl.Close()

	if err := fl.Lock(); err != nil {
		panic(err)
	}

	otherPid, err := os.ReadFile(pidPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
	if oth := strings.TrimSpace(string(otherPid)); oth != "" {
		if err := exec.Command("kill", oth).Run(); err != nil {
			if exitErr := (&exec.ExitError{}); !errors.As(err, &exitErr) {
				panic(err)
			}
		}
	}

	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		panic(err)
	}

	if err := fl.Unlock(); err != nil {
		panic(err)
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
		panic(err)
	}
}
