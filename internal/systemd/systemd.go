// Package systemd wraps systemctl and journalctl.
package systemd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

type Unit struct {
	Name        string
	Load        string
	Active      string
	Sub         string
	Description string
}

// Available reports whether the machine was booted with systemd.
func Available() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func Services(ctx context.Context) ([]Unit, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--all",
		"--no-legend", "--no-pager", "--plain").Output()
	if err != nil {
		return nil, err
	}
	return parseUnits(out), nil
}

func parseUnits(b []byte) []Unit {
	var units []Unit
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		u := Unit{Name: f[0], Load: f[1], Active: f[2], Sub: f[3]}
		if len(f) > 4 {
			u.Description = strings.Join(f[4:], " ")
		}
		units = append(units, u)
	}
	return units
}

// Action runs start, stop or restart. It never prompts for a password,
// so without the right privileges it just fails.
func Action(ctx context.Context, unit, action string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return errors.New("unknown action " + action)
	}
	out, err := exec.CommandContext(ctx, "systemctl", "--no-ask-password", action, unit).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

// Journal follows the journal, optionally for a single unit, and calls
// lines for every line until ctx is cancelled.
func Journal(ctx context.Context, unit string, lines func(string)) error {
	args := []string{"--no-pager", "-f", "-n", "200"}
	if unit != "" {
		args = append(args, "-u", unit)
	}
	return follow(ctx, exec.CommandContext(ctx, "journalctl", args...), lines)
}

// Tail follows a plain log file, for machines without a journal.
func Tail(ctx context.Context, path string, lines func(string)) error {
	return follow(ctx, exec.CommandContext(ctx, "tail", "-n", "200", "-F", path), lines)
}

func follow(ctx context.Context, cmd *exec.Cmd, lines func(string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines(sc.Text())
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return errors.New(msg)
	}
	return err
}
