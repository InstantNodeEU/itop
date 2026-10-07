package systemd

import "testing"

func TestParseUnits(t *testing.T) {
	in := []byte(`cron.service          loaded active   running Regular background program processing daemon
nginx.service         loaded failed   failed  A high performance web server
not-found.service     not-found inactive dead  not-found.service
short.service loaded
`)
	units := parseUnits(in)
	if len(units) != 3 {
		t.Fatalf("got %d units", len(units))
	}
	if u := units[1]; u.Name != "nginx.service" || u.Active != "failed" || u.Description != "A high performance web server" {
		t.Errorf("got %+v", u)
	}
	if units[2].Load != "not-found" {
		t.Errorf("got %+v", units[2])
	}
}
