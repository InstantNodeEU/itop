<p align="center"><img src="assets/banner.png" alt="itop"></p>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-e3b341?style=flat-square" alt="license MIT">
  <img src="https://img.shields.io/badge/written%20in-Go-e3b341?style=flat-square" alt="Go">
  <a href="https://github.com/instantnodeeu/itop/releases"><img src="https://img.shields.io/github/v/release/instantnodeeu/itop?style=flat-square&color=e3b341" alt="release"></a>
  <a href="https://github.com/instantnodeeu/itop/stargazers"><img src="https://img.shields.io/github/stars/instantnodeeu/itop?style=flat-square&color=e3b341" alt="stars"></a>
  <a href="https://instantnode.eu"><img src="https://img.shields.io/badge/by-InstantNode-e3b341?style=flat-square" alt="by InstantNode"></a>
</p>

A terminal system monitor for Linux servers. It is htop with the stuff I
kept opening in other panes bolted on: Docker containers, systemd services,
listening ports and logs, all in one window.

- **Processes** with CPU, memory, sorting, filtering and kill
- **Docker** containers with live CPU/memory, start/stop/restart and logs
- **Services** from systemd, failed units on top, restart and journal
- **Ports** that are listening, which process owns them, and whether they
  are bound to something other than loopback
- **Logs** follows the journal, a unit, or a container

The header shows per core CPU, memory, swap, load, uptime, root disk and
network throughput.

It reads `/proc` directly, talks to the Docker socket over plain HTTP and
shells out to `systemctl`/`journalctl`. No agent, no config file, one static
binary. If Docker or systemd is missing the tab just says so.

![itop processes tab](assets/processes.png)

<details>
<summary>Ports and services</summary>

![itop ports tab](assets/ports.png)
![itop services tab](assets/services.png)

</details>

## Install

```sh
curl -Lo itop https://github.com/instantnodeeu/itop/releases/latest/download/itop-linux-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
chmod +x itop
sudo mv itop /usr/local/bin/
```

Or with Go 1.24+:

```sh
go install github.com/instantnodeeu/itop@latest
```

## Usage

```sh
itop          # refresh every 2s
itop -d 1s    # faster
sudo itop     # see every process's ports and all journal entries
```

Running without root works fine. You just won't see which process owns
sockets of other users, and service actions will fail unless polkit allows
them. Docker needs your user in the `docker` group.

## Keys

| Key | |
|---|---|
| `1`-`5`, `Tab` | switch tab |
| `/` | filter the current tab, `Esc` clears |
| `?` | help |
| `q`, `F10` | quit |

Processes: `c` `m` `p` `n` sort by CPU, memory, PID, name (press again to
reverse), `k` or `F9` to send TERM/KILL.

Docker and Services: `s` start, `x` stop, `r` restart, `l` or `Enter` to
follow logs. `a` toggles inactive services.

Ports: `Enter` jumps to the owning process.

Logs: `g`/`G` top and end, `s` goes back to the system log.

## Building

```sh
make          # ./itop
make test
make dist     # static amd64 and arm64 binaries in dist/
```

## License

MIT
