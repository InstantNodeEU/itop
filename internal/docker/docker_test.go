package docker

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDemux(t *testing.T) {
	var in bytes.Buffer
	frame := func(stream byte, s string) {
		h := []byte{stream, 0, 0, 0, 0, 0, 0, byte(len(s))}
		in.Write(h)
		in.WriteString(s)
	}
	frame(1, "hello\n")
	frame(2, "oops\n")
	var out bytes.Buffer
	if err := demux(&in, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello\noops\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestStatsParse(t *testing.T) {
	raw := `{
		"cpu_stats": {"cpu_usage": {"total_usage": 2000}, "system_cpu_usage": 20000, "online_cpus": 4},
		"precpu_stats": {"cpu_usage": {"total_usage": 1000}, "system_cpu_usage": 10000},
		"memory_stats": {"usage": 1000, "limit": 8000, "stats": {"inactive_file": 200}}
	}`
	var s statsJSON
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	st := s.parse()
	if st.CPU != 40 || st.Mem != 800 || st.MemLimit != 8000 {
		t.Errorf("got %+v", st)
	}
}

func TestPortList(t *testing.T) {
	var c Container
	json.Unmarshal([]byte(`{"Ports":[
		{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},
		{"IP":"::","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},
		{"PrivatePort":5432,"Type":"tcp"}]}`), &c)
	if got := c.PortList(); got != "8080->80/tcp, 5432/tcp" {
		t.Errorf("got %q", got)
	}
}
