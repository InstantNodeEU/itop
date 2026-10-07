package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/instantnodeeu/itop/internal/sys"
	"github.com/rivo/tview"
)

type header struct {
	*tview.TextView

	prevCores []sys.CPUTimes
	prevRx    uint64
	prevTx    uint64
	prevNet   time.Time
}

type headerData struct {
	cores    []float64
	mem      sys.Mem
	load     [3]float64
	up       float64
	diskTot  uint64
	diskUsed uint64
	rxRate   float64
	txRate   float64
}

func newHeader() *header {
	h := &header{TextView: tview.NewTextView()}
	h.SetDynamicColors(true).SetWrap(false)
	return h
}

func (h *header) update() headerData {
	var d headerData
	_, cores, err := sys.ReadCPU()
	if err == nil {
		for i, c := range cores {
			var prev sys.CPUTimes
			if i < len(h.prevCores) {
				prev = h.prevCores[i]
			}
			d.cores = append(d.cores, c.Usage(prev))
		}
		h.prevCores = cores
	}
	d.mem, _ = sys.ReadMem()
	d.load, _ = sys.LoadAvg()
	d.up, _ = sys.Uptime()
	d.diskTot, d.diskUsed, _ = sys.DiskUsage("/")

	rx, tx, err := sys.NetBytes()
	now := time.Now()
	if err == nil {
		if !h.prevNet.IsZero() && rx >= h.prevRx && tx >= h.prevTx {
			secs := now.Sub(h.prevNet).Seconds()
			d.rxRate = float64(rx-h.prevRx) / secs
			d.txRate = float64(tx-h.prevTx) / secs
		}
		h.prevRx, h.prevTx, h.prevNet = rx, tx, now
	}
	return d
}

// render returns the number of lines the header needs.
func (h *header) render(d headerData) int {
	_, _, width, _ := h.GetInnerRect()
	if width < 20 {
		width = 80
	}
	cols := 1
	switch {
	case width >= 160:
		cols = 4
	case width >= 80:
		cols = 2
	}
	cols = max(min(cols, len(d.cores)), 1)
	cellW := width / cols

	var b strings.Builder
	rows := (len(d.cores) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			// fill column-wise like htop, so cpu 0..n-1 run down the first column
			i := c*rows + r
			if i >= len(d.cores) {
				break
			}
			fmt.Fprintf(&b, "[aqua]%3d[-]%s ", i, bar(d.cores[i], cellW-7, ""))
		}
		b.WriteByte('\n')
	}

	half := width/2 - 6
	memPct, swapPct := 0.0, 0.0
	if d.mem.Total > 0 {
		memPct = float64(d.mem.Used()) / float64(d.mem.Total) * 100
	}
	if d.mem.SwapTotal > 0 {
		swapPct = float64(d.mem.SwapUsed()) / float64(d.mem.SwapTotal) * 100
	}
	fmt.Fprintf(&b, "[aqua]Mem[-]%s  [aqua]Swp[-]%s\n",
		bar(memPct, half, human(d.mem.Used())+"/"+human(d.mem.Total)),
		bar(swapPct, half, human(d.mem.SwapUsed())+"/"+human(d.mem.SwapTotal)))

	diskPct := 0.0
	if d.diskTot > 0 {
		diskPct = float64(d.diskUsed) / float64(d.diskTot) * 100
	}
	fmt.Fprintf(&b, "[aqua]Load[-] %.2f %.2f %.2f  [aqua]Up[-] %s  [aqua]Disk /[-] %s/%s (%.0f%%)  [aqua]Net[-] rx %s/s tx %s/s",
		d.load[0], d.load[1], d.load[2], uptime(d.up),
		human(d.diskUsed), human(d.diskTot), diskPct,
		human(uint64(d.rxRate)), human(uint64(d.txRate)))
	if os.Geteuid() != 0 {
		b.WriteString("  [yellow]not root, some details hidden[-]")
	}
	h.SetText(b.String())
	return rows + 2
}
