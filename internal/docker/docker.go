// Package docker is a small client for the parts of the Engine API itop needs.
package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Timeout is meant for the short request/response calls.
const Timeout = 5 * time.Second

type Client struct {
	hc *http.Client
}

type Container struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
	Ports  []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

func (c Container) Name() string {
	if len(c.Names) == 0 {
		return c.ID[:12]
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

// PortList renders published ports the way `docker ps` does, minus the noise.
func (c Container) PortList() string {
	seen := map[string]bool{}
	var out []string
	for _, p := range c.Ports {
		var s string
		if p.PublicPort != 0 {
			s = fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type)
		} else {
			s = fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

type Stats struct {
	CPU      float64
	Mem      uint64
	MemLimit uint64
}

// SocketPath honours DOCKER_HOST when it points at a unix socket.
func SocketPath() string {
	if h := os.Getenv("DOCKER_HOST"); strings.HasPrefix(h, "unix://") {
		return strings.TrimPrefix(h, "unix://")
	}
	return "/var/run/docker.sock"
}

func New(socket string) *Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	return &Client{hc: &http.Client{Transport: tr}}
}

func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		var e struct{ Message string }
		json.NewDecoder(resp.Body).Decode(&e)
		if e.Message == "" {
			e.Message = resp.Status
		}
		return nil, errors.New(e.Message)
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/_ping")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) Containers(ctx context.Context) ([]Container, error) {
	var out []Container
	err := c.getJSON(ctx, "/containers/json?all=1", &out)
	return out, err
}

// Stats takes one sample. Without one-shot the daemon fills in precpu_stats,
// which costs about a second but gives us a real cpu percentage.
func (c *Client) Stats(ctx context.Context, id string) (Stats, error) {
	var raw statsJSON
	if err := c.getJSON(ctx, "/containers/"+url.PathEscape(id)+"/stats?stream=false", &raw); err != nil {
		return Stats{}, err
	}
	return raw.parse(), nil
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint64 `json:"online_cpus"`
}

type statsJSON struct {
	CPU      cpuStats `json:"cpu_stats"`
	PreCPU   cpuStats `json:"precpu_stats"`
	MemStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

func (s statsJSON) parse() Stats {
	var out Stats
	dc := float64(s.CPU.CPUUsage.TotalUsage) - float64(s.PreCPU.CPUUsage.TotalUsage)
	ds := float64(s.CPU.SystemUsage) - float64(s.PreCPU.SystemUsage)
	if dc > 0 && ds > 0 {
		n := float64(s.CPU.OnlineCPUs)
		if n == 0 {
			n = 1
		}
		out.CPU = dc / ds * n * 100
	}
	// same as the docker cli: page cache does not count as used
	cache := s.MemStats.Stats["inactive_file"]
	if v, ok := s.MemStats.Stats["total_inactive_file"]; ok {
		cache = v
	}
	out.Mem = s.MemStats.Usage
	if cache < out.Mem {
		out.Mem -= cache
	}
	out.MemLimit = s.MemStats.Limit
	return out
}

// Action runs start, stop or restart on a container.
func (c *Client) Action(ctx context.Context, id, action string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	resp, err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/"+action)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Logs follows a container's output and sends it line by line until ctx is done.
func (c *Client) Logs(ctx context.Context, id string, lines func(string)) error {
	var info struct {
		Config struct{ Tty bool }
	}
	if err := c.getJSON(ctx, "/containers/"+url.PathEscape(id)+"/json", &info); err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/logs?stdout=1&stderr=1&follow=1&tail=200")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r io.Reader = resp.Body
	if !info.Config.Tty {
		pr, pw := io.Pipe()
		go func() { pw.CloseWithError(demux(resp.Body, pw)) }()
		r = pr
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines(sc.Text())
	}
	if ctx.Err() != nil {
		return nil
	}
	return sc.Err()
}

// demux strips the 8 byte frame headers docker puts on non-tty streams.
func demux(r io.Reader, w io.Writer) error {
	var hdr [8]byte
	for {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		n := int64(binary.BigEndian.Uint32(hdr[4:]))
		if _, err := io.CopyN(w, r, n); err != nil {
			return err
		}
	}
}
