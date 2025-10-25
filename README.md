## Pinger

Small macOS menu bar utility that monitors network connectivity to a target host and shows:

- Latest round‑trip time history as a live mini chart in the menu bar
- A dropdown of the most recent 20 samples with timestamps and latency
- Configurable target and probe mode through the UI

![Pinger menubar](docs/menubar.png)

### Features

- **Live tray icon**: bars update every ping; color animates while a ping is in flight
- **History buffer**: keeps recent samples in a ring buffer (default 60)
- **Quick overview**: open the tray to see the last 20 results; failures are marked
- **Configurable target**: change the ping target directly from the menu bar
- **Multiple probe modes**: 
  - **ICMP Mode**: Traditional ping using ICMP packets (unprivileged, no root required)
  - **HTTP Mode**: TCP connection probe to port 80 (useful when ICMP is blocked)
- **Persistent settings**: target and probe mode are saved and restored between sessions

### Build and run

Requirements: Go 1.22+

```bash
git clone https://github.com/nenad/pinger
cd pinger
go build -o pinger .
./pinger
```

You can also run directly:

```bash
go run .
```

### Configuration

Pinger stores its configuration in `~/.config/pinger/config.json` with the following settings:

- **target**: The host to monitor (default: `1.1.1.1`)
- **probe_mode**: Either `ICMP` or `HTTP` (default: `ICMP`)

You can change these settings directly from the menu bar:
- Click **"Change Target..."** to set a new target address
- Select **"ICMP Mode"** or **"HTTP Mode"** to switch probe methods

The ping interval (1 second) and timeout (2 seconds) are currently fixed but can be modified in the source code if needed.

### Notes

- **ICMP Mode** uses unprivileged ICMP via `prometheus-community/pro-bing` to avoid root requirements.
- **HTTP Mode** uses TCP connection attempts to port 80, which works even when ICMP is blocked by firewalls.
- Menu bar integration is powered by `getlantern/systray`.
- Configuration dialog uses native macOS AppleScript dialogs.

### License

MIT


