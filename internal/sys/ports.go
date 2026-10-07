package sys

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Port struct {
	Proto string // tcp, tcp6, udp, udp6
	Addr  string
	Port  int
	Inode uint64
	UID   uint32
	PID   int
	Name  string
}

const tcpListen = "0A"

// ListenPorts returns listening TCP sockets and bound UDP sockets,
// with the owning process filled in where we can see it.
func ListenPorts() ([]Port, error) {
	var out []Port
	var firstErr error
	for _, proto := range []string{"tcp", "tcp6", "udp", "udp6"} {
		s, err := readFile("net", proto)
		if err != nil {
			if firstErr == nil && !os.IsNotExist(err) {
				firstErr = err
			}
			continue
		}
		out = append(out, parseNet(proto, s)...)
	}
	owners := socketOwners()
	for i := range out {
		if pid, ok := owners[out[i].Inode]; ok {
			out[i].PID = pid
			if c, err := readFile(strconv.Itoa(pid), "comm"); err == nil {
				out[i].Name = strings.TrimSpace(c)
			}
		}
	}
	return out, firstErr
}

func parseNet(proto, s string) []Port {
	var out []Port
	udp := strings.HasPrefix(proto, "udp")
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Scan() // header
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 10 {
			continue
		}
		if !udp && f[3] != tcpListen {
			continue
		}
		// connected udp sockets have a remote port, skip them
		if udp && !strings.HasSuffix(f[2], ":0000") {
			continue
		}
		ip, port, err := parseAddr(f[1])
		if err != nil {
			continue
		}
		uid, _ := strconv.ParseUint(f[7], 10, 32)
		out = append(out, Port{
			Proto: proto,
			Addr:  ip.String(),
			Port:  port,
			Inode: atou(f[9]),
			UID:   uint32(uid),
		})
	}
	return out
}

// parseAddr decodes "0100007F:0035". The address is stored as host-order
// 32-bit words, so on little endian each group of 4 bytes is reversed.
func parseAddr(s string) (net.IP, int, error) {
	h, p, ok := strings.Cut(s, ":")
	if !ok {
		return nil, 0, fmt.Errorf("bad address %q", s)
	}
	b, err := hex.DecodeString(h)
	if err != nil || (len(b) != 4 && len(b) != 16) {
		return nil, 0, fmt.Errorf("bad address %q", s)
	}
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return nil, 0, err
	}
	return net.IP(b), int(port), nil
}

// socketOwners maps socket inodes to pids. Without root this only
// covers our own processes.
func socketOwners() map[uint64]int {
	m := map[uint64]int{}
	d, err := os.Open(procPath())
	if err != nil {
		return m
	}
	names, _ := d.Readdirnames(-1)
	d.Close()
	for _, n := range names {
		pid, err := strconv.Atoi(n)
		if err != nil {
			continue
		}
		fdDir := procPath(n, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(fdDir + "/" + fd.Name())
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			ino := atou(strings.TrimSuffix(link[len("socket:["):], "]"))
			if _, seen := m[ino]; !seen {
				m[ino] = pid
			}
		}
	}
	return m
}
