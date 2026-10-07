package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/instantnodeeu/itop/internal/docker"
	"github.com/instantnodeeu/itop/internal/systemd"
	"github.com/rivo/tview"
)

type logsPanel struct {
	*tview.TextView
	app    *app
	source string
	cancel context.CancelFunc
	gen    int

	mu      sync.Mutex
	pending []string
}

func newLogsPanel(a *app) *logsPanel {
	p := &logsPanel{TextView: tview.NewTextView(), app: a}
	p.SetDynamicColors(false).SetScrollable(true).SetMaxLines(5000)
	return p
}

func (p *logsPanel) title() string { return "Logs" }

func (p *logsPanel) hints() string {
	return p.source + "  G end  g top  s system log"
}

// Logs stream on their own, nothing to poll.
func (p *logsPanel) update() func() { return nil }

func (p *logsPanel) setFilter(string) {}

func (p *logsPanel) key(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Rune() == 's' {
		p.followSystem()
		return nil
	}
	return ev
}

func (p *logsPanel) followUnit(unit string) {
	p.start("journal: "+unit, func(ctx context.Context, out func(string)) error {
		return systemd.Journal(ctx, unit, out)
	})
}

func (p *logsPanel) followContainer(cli *docker.Client, id, name string) {
	p.start("docker: "+name, func(ctx context.Context, out func(string)) error {
		return cli.Logs(ctx, id, out)
	})
}

// followSystem picks the journal if there is one, otherwise a classic
// syslog file.
func (p *logsPanel) followSystem() {
	if _, err := exec.LookPath("journalctl"); err == nil && systemd.Available() {
		p.start("journal", func(ctx context.Context, out func(string)) error {
			return systemd.Journal(ctx, "", out)
		})
		return
	}
	for _, f := range []string{"/var/log/syslog", "/var/log/messages"} {
		if fh, err := os.Open(f); err == nil {
			fh.Close()
			p.start(f, func(ctx context.Context, out func(string)) error {
				return systemd.Tail(ctx, f, out)
			})
			return
		}
	}
	p.stop()
	p.source = "none"
	p.SetText("No system log found. Pick a container or service and press l.")
}

func (p *logsPanel) stop() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.gen++
}

func (p *logsPanel) start(source string, run func(context.Context, func(string)) error) {
	p.stop()
	gen := p.gen
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.source = source
	p.Clear()
	p.ScrollToEnd()
	p.mu.Lock()
	p.pending = nil
	p.mu.Unlock()
	p.app.switchTo(p)

	go func() {
		err := run(ctx, func(line string) {
			p.mu.Lock()
			p.pending = append(p.pending, line)
			p.mu.Unlock()
		})
		if err != nil {
			p.mu.Lock()
			p.pending = append(p.pending, "-- "+err.Error())
			p.mu.Unlock()
		}
	}()

	// flush in batches, a busy container can easily print thousands of lines a second
	go func() {
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			p.mu.Lock()
			lines := p.pending
			p.pending = nil
			p.mu.Unlock()
			if len(lines) == 0 {
				continue
			}
			p.app.QueueUpdateDraw(func() {
				if p.gen != gen {
					return
				}
				p.Write([]byte(strings.Join(lines, "\n") + "\n"))
			})
		}
	}()
}
