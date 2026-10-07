package sys

import (
	"os"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	ProcRoot = "testdata/proc"
	PasswdPath = "testdata/passwd"
	os.Exit(m.Run())
}

func TestReadCPU(t *testing.T) {
	all, cores, err := ReadCPU()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 2 {
		t.Fatalf("got %d cores, want 2", len(cores))
	}
	if all.Total != 10000 || all.Busy != 1500 {
		t.Errorf("all = %+v", all)
	}
	next := CPUTimes{Busy: all.Busy + 50, Total: all.Total + 100}
	if u := next.Usage(all); u != 50 {
		t.Errorf("usage = %v, want 50", u)
	}
	if u := all.Usage(next); u != 0 {
		t.Errorf("usage going backwards = %v, want 0", u)
	}
}

func TestReadMem(t *testing.T) {
	m, err := ReadMem()
	if err != nil {
		t.Fatal(err)
	}
	if m.Total != 16384000*1024 || m.Used() != 8192000*1024 {
		t.Errorf("mem = %+v", m)
	}
	if m.SwapUsed() != 1024000*1024 {
		t.Errorf("swap used = %d", m.SwapUsed())
	}
}

func TestLoadAndUptime(t *testing.T) {
	l, err := LoadAvg()
	if err != nil || l != [3]float64{0.52, 0.48, 0.40} {
		t.Errorf("load = %v, %v", l, err)
	}
	u, err := Uptime()
	if err != nil || u != 93784.12 {
		t.Errorf("uptime = %v, %v", u, err)
	}
}

func TestNetBytes(t *testing.T) {
	rx, tx, err := NetBytes()
	if err != nil {
		t.Fatal(err)
	}
	if rx != 1002000 || tx != 250500 {
		t.Errorf("rx=%d tx=%d", rx, tx)
	}
}

func TestParseStat(t *testing.T) {
	p, err := parseStat("77 (weird ) name) R 1 77 77 0 -1 0 0 0 0 0 30 12 0 0 20 5 1 0 100 0 25 0")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "weird ) name" || p.State != "R" || p.Ticks != 42 || p.Nice != 5 {
		t.Errorf("got %+v", p)
	}
	if p.RSS != 25*pageSize {
		t.Errorf("rss = %d", p.RSS)
	}
	if _, err := parseStat("garbage"); err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestProcs(t *testing.T) {
	ps, err := Procs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("got %d procs", len(ps))
	}
	p := ps[0]
	if p.PID != 4242 || p.Name != "tmux: server" || p.Cmd != "tmux new -s main" || p.Ticks != 200 {
		t.Errorf("got %+v", p)
	}
}

func TestParseAddr(t *testing.T) {
	cases := map[string]string{
		"0100007F:0035":                         "127.0.0.1",
		"00000000:0016":                         "0.0.0.0",
		"00000000000000000000000001000000:0277": "::1",
		"B80D0120000000000000000001000000:01BB": "2001:db8::1",
	}
	for in, want := range cases {
		ip, _, err := parseAddr(in)
		if err != nil || ip.String() != want {
			t.Errorf("parseAddr(%q) = %v, %v; want %s", in, ip, err, want)
		}
	}
	if _, port, _ := parseAddr("0100007F:1F90"); port != 8080 {
		t.Errorf("port = %d", port)
	}
	if _, _, err := parseAddr("zz:0035"); err == nil {
		t.Error("expected error")
	}
}

func TestListenPorts(t *testing.T) {
	ports, err := ListenPorts()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range ports {
		got[p.Proto+" "+p.Addr+":"+strconv.Itoa(p.Port)] = true
	}
	want := []string{"tcp 0.0.0.0:22", "tcp 127.0.0.1:8080", "tcp6 ::1:631", "udp 127.0.0.53:53"}
	if len(ports) != len(want) {
		t.Errorf("got %d ports: %v", len(ports), got)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %s", w)
		}
	}
}

func TestUsername(t *testing.T) {
	if u := Username(0); u != "root" {
		t.Errorf("uid 0 = %q", u)
	}
	if u := Username(31337); u != "31337" {
		t.Errorf("unknown uid = %q", u)
	}
}
