package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Renders every tab on a fake screen against the real /proc. Mostly a
// smoke test that nothing panics when docker or systemd are missing.
func TestRenderTabs(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(120, 40)

	a := newApp(time.Second)
	a.SetScreen(screen)
	go a.Run()
	defer a.Stop()

	want := []string{"COMMAND", "IMAGE", "UNIT", "PROCESS", ""}
	for i, w := range want {
		done := make(chan struct{})
		a.QueueUpdateDraw(func() { a.selectTab(i) })
		hd := a.header.update()
		apply := a.current().update()
		a.QueueUpdateDraw(func() {
			a.layout.ResizeItem(a.header, a.header.render(hd), 0)
			if apply != nil {
				apply()
			}
			close(done)
		})
		<-done
		a.QueueUpdateDraw(func() {})
		time.Sleep(50 * time.Millisecond)

		text := screenText(screen)
		if !strings.Contains(text, "Mem") || !strings.Contains(text, a.panels[i].title()) {
			t.Errorf("tab %d: header or tab bar missing:\n%s", i, text)
		}
		if w != "" && !strings.Contains(text, w) {
			t.Errorf("tab %d: %q not on screen:\n%s", i, w, text)
		}
	}
}

func screenText(s tcell.SimulationScreen) string {
	cells, w, _ := s.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteByte(' ')
		}
		if (i+1)%w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
