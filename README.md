# Universal GameSpy, Quake & Retro Master Server

A unified, high-performance master server daemon written in **Go**, paired with an authentic **The All-Seeing Eye (ASE)** themed web dashboard built with **Next.js 16 (App Router) + Tailwind CSS + shadcn/ui**.

Designed as a modern drop-in replacement for retro multiplayer master servers (GameSpy v0/v1, Quake 2, Quake 3 / idTech 3, Kingpin, Half-Life, Battlefield 1942, and 4,600+ cataloged classic games), with automated ingestion and multi-threaded UDP status probing from the [`retro-server-mirror`](https://github.com/shavargo/retro-server-mirror) registry.

---

## The All-Seeing Eye (ASE) Web Launcher

The web frontend recreates the iconic look, ergonomics, and responsiveness of the legendary 2000s **The All-Seeing Eye**:
- **3-Pane Desktop Layout**: Collapsible directory tree on the left, sortable server grid in the center, and dual combat roster / server telemetry inspector below.
- **Interactive Multi-Column Sorting**: Sort by Ping, Player count, Game, Server Name, Map, Gametype, or IP.
- **Collapsible Server CVars**: Clean high-level telemetry with an on-demand dropdown accordion for all raw server engine CVars.
- **Global Keyboard Navigation**: Navigate servers with `↑`/`↓` (or `j`/`k`), hit `Enter` to connect, `/` to filter, and `Escape` to clear.
- **Multi-Connect Suite**: One-click raw IP copy, in-game console string (`/connect <ip>:<port>`), and direct Steam protocol launch (`steam://connect/<ip>:<port>`) for supported games.
- **Server List Export**: Download `.txt` server lists formatted for direct script and game config ingestion.
- **Procedural Web Audio API SFX**: 100% royalty-free, synthesized retro acoustic cues (clicks, chimes, sweeps) generated entirely in-code with zero copyrighted audio files.

---

## Architecture Overview

```
                      ┌──────────────────────────────────────────────┐
                      │              VPS / Host Network              │
                      └──────────────────────┬───────────────────────┘
                                             │
       ┌───────────────────┬─────────────────┴───────┬───────────────────┐
       │                   │                         │                   │
 [UDP 27900]          [UDP 27950]               [TCP 28900]         [HTTP 80/8080]
Dual Protocol        KPQ3 / DarkPlaces         GameSpy v0 List      Next.js 16 + shadcn
Packet Demuxer        Server Browser             & 333Networks        Public Dashboard
       │                   │                         │                   │
 ├── 0x5C ('\'):           │                   ├── \list\                │
 │   GameSpy Heartbeats    │                   │   (UT, KP, etc.)        │
 └── 0xFF (Quake 2):       │                   └── \sync\                │
     In-Game Browser       │                       (333Networks)         │
                           └────────────┬────────────┘                   │
                                        │                                │
                          ┌─────────────┴─────────────┐                  │
                          │   Thread-Safe In-Memory   │                  │
                          │     Server Registry       │                  │
                          │ (Atomic Snapshot Pointer) ├──────────────────┘
                          └─────────────┬─────────────┘      (JSON via :8085 API)
                                        │
                          ┌─────────────┴─────────────┐
                          │    Background Prober      │
                          │ (Pings UDP \status\ every │
                          │  15s to verify live state)│
                          └───────────────────────────┘
```

---

## Key Features

1. **Dual-Protocol Packet Demuxer on Port 27900 (UDP)**:
   * **GameSpy v0**: Packets starting with `\` (`0x5C`) are parsed as standard GameSpy heartbeats (`\heartbeat\<port>\gamename\<game>`).
   * **Quake 2 / Kingpin**: Packets starting with `0xFF 0xFF 0xFF 0xFF` (`getservers`, `query\n`, `heartbeat\n`) return native binary server lists to in-game Kingpin client browsers.
2. **KingpinQ3 / DarkPlaces on Port 27950 (UDP)**:
   * Supports `getservers KingpinQ3-1` and DarkPlaces protocol.
3. **GameSpy Server List Server on Port 28900 (TCP)**:
   * Responds to `\list\cmp\` and `\list\` queries for all supported games.
   * Full compatibility with the **333Networks Master Server federation** via `\sync\`.
4. **2,960+ Game Catalog Embedded**:
   * Pre-configured with validation ciphers from the 333Networks game database.
   * Kingpin, Deus Ex, Rune, and others automatically bypass `\secure\` challenges to accept dedicated servers without dropped connections.
5. **Anti-Abuse & Public Hardening**:
   * **Zero-Load Snapshot Cache**: The public dashboard reads from an in-memory atomic pointer ($< 1\text{ms}$ response time). Web scraping or DDoS will not cause master server ping spikes.
   * **Probe Verification Guard**: Heartbeats are only displayed once the prober confirms the server is reachable via a direct UDP status query.
   * **Zero-XSS Sanitization**: Quake color codes (`^1`, `^7`) are automatically stripped, strings are length-capped, and HTML entities are escaped.
   * **Private IP Filtering**: Filters out `127.0.0.1` and RFC1918 addresses from public listing.

---

## Ports & Protocols

| Port | Protocol | Purpose |
| :--- | :--- | :--- |
| **`27900`** | UDP | Dual GameSpy heartbeats (`\heartbeat\`) + Quake 2 in-game browser (`query\n`) |
| **`27950`** | UDP | KingpinQ3 / DarkPlaces client browser |
| **`28900`** | TCP | GameSpy server list query (`\list\`) + 333Networks sync (`\sync\`) |
| **`8085`** | TCP | Internal REST API (communicates between Go daemon and Next.js frontend) |
| **`80 / 8080`**| TCP | Public Next.js + shadcn/ui live server browser dashboard |

---

## Quickstart: Zero-Clone Docker Deployment

You **do not need to clone this repository** to run the complete master server and web launcher. Pre-built multi-architecture container images are published to the GitHub Container Registry (`ghcr.io`).

### Option 1: Single `docker run` Command (Zero Clone)

Run the unified all-in-one container directly:

```bash
docker run -d \
  --name all-seeing-eye \
  -p 27900:27900/udp \
  -p 27950:27950/udp \
  -p 28900:28900/tcp \
  -p 3000:3000 \
  --restart unless-stopped \
  ghcr.io/shavargo/all-seeing-eye:latest
```

Open [http://localhost:3000](http://localhost:3000) to view the live All-Seeing Eye launcher.

### Option 2: Docker Compose One-Liner (Zero Clone)

Run using docker compose without cloning files:

```bash
curl -sSL https://raw.githubusercontent.com/shavargo/all-seeing-eye/master/docker-compose.yml | docker compose -f - up -d
```

---

## Deployment from Cloned Repository

To view logs:
```bash
docker compose logs -f
```

To stop:
```bash
docker compose down
```

---

## Local Development

### 1. Run the Go Master Daemon
```bash
go run cmd/master/main.go
```
The master server will listen on UDP `27900`, UDP `27950`, TCP `28900`, and expose the internal API on `http://127.0.0.1:8085`.

### 2. Run the Next.js Frontend
```bash
cd web
npm install
npm run dev
```
Open [http://localhost:3000](http://localhost:3000) to view the live dashboard.

---

## Attribution & Acknowledgements

- Server address registries and multi-master mirrors are ingested from [retro-server-mirror](https://github.com/shavargo/retro-server-mirror).
- Game definitions and metadata originate from [lookout-games](https://github.com/molycode/lookout-games) (MIT, Copyright (c) 2026 moly).
- Master server protocols and network data from [OpenSpy](https://openspy.net/) and [333networks](https://333networks.com/).

---

## License

This project is licensed under the [MIT License](LICENSE) - see the [LICENSE](LICENSE) file for details.
