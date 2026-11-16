# OpenMaster

A unified, high-performance master server daemon written in **Go**, paired with a lightweight web directory powered by **NGINX**.

Designed as an open, reliable drop-in replacement for retro multiplayer master servers (GameSpy v0/v1, Quake 2, Quake 3 / idTech 3, Kingpin, Half-Life, Battlefield 1942, and 4,800+ cataloged classic games), operating in **100% passive directory mode** with zero unsolicited outbound UDP probing.

---

## Web Directory & Connection Guide

The web interface is served via an ultra-lightweight NGINX container (`~30 MB` image size) with sub-second startup:
- **Instant Search & Real-time Game Filtering**: Filter across tens of thousands of retro servers by game, server name, or IP address in real-time.
- **Connection Guide**: Live master server setup instructions and ports for GameSpy TCP (`:28900`), Quake 2 UDP (`:27900`), and idTech 3 UDP (`:27950`).
- **REST API Reverse Proxy**: Direct reverse-proxying of `/api/v1/stats`, `/api/v1/servers`, and `/api/v1/games` without Node.js overhead.
- **Clean Monospace Presentation**: Clean, lightweight table with single-click address copying.

---

## Architecture Overview

```
                      ┌──────────────────────────────────────────────┐
                      │              VPS / Host Network              │
                      └──────────────────────┬───────────────────────┘
                                             │
       ┌───────────────────┬─────────────────┴───────┬───────────────────┐
       │                   │                         │                   │
 [UDP 27900]          [UDP 27950]               [TCP 28900]         [HTTP 80/3000]
Dual Protocol        KPQ3 / idTech 3           GameSpy v0 List      NGINX Reverse Proxy
Packet Demuxer        Server Browser             & 333Networks      & Server Directory
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
                          │    Passive Daily Mirror   │
                          │  (Imports verified retro  │
                          │   servers with 0 probes)  │
                          └───────────────────────────┘
```

---

## Key Features

1. **Dual-Protocol Packet Demuxer on Port 27900 (UDP)**:
   * **GameSpy v0**: Packets starting with `\` (`0x5C`) are parsed as standard GameSpy heartbeats (`\heartbeat\<port>\gamename\<game>`).
   * **Quake 2 / Kingpin**: Packets starting with `0xFF 0xFF 0xFF 0xFF` (`getservers`, `query\n`, `heartbeat\n`) return native binary server lists to in-game Quake 2 and Kingpin client browsers.
2. **Quake 3 / idTech 3 & Multi-Port Activision Spinoffs (UDP)**:
   * Handles native `\xff\xff\xff\xffgetservers` queries simultaneously across all standard engine ports:
     * Quake 3 Arena / OpenArena (`:27950`)
     * Call of Duty (`:20510`), CoD:UO (`:20610`), CoD 2 (`:20710`), CoD 4 (`:20810`)
     * Star Wars Jedi Knight II (`:28060`) & Jedi Academy (`:29060`)
     * Soldier of Fortune II (`:20110`)
     * Star Trek Elite Force 1 (`:27953`) & Elite Force 2 (`:29250`)
     * DarkPlaces / Xonotic / Nexuiz (`:27777`)
3. **Unreal Tournament 2004 & 2003 on Port 28902 (TCP)**:
   * Full native support for `[IpDrv.MasterServerLink]` and `MasterServerClient` binary protocols.
   * Delivers length-prefixed challenge auth, `APPROVED`/`VERIFIED` handshakes, MOTD delivery, and binary server list feeds to the in-game server browser.
   * Accepts dedicated server uplinks and refresh packets.
4. **GameSpy Master List Server on Ports 28900 & 28910 (TCP)**:
   * Responds to `\list\cmp\` and `\list\` queries for UT99, Rune, Deus Ex, and GameSpy v0/v1 clients.
   * Full compatibility with the **333Networks Master Server federation** via `\sync\`.
5. **4,800+ Game Catalog Embedded**:
   * Pre-configured with validation ciphers from the GameSpy and 333Networks catalog.
   * Unreal, Quake, Deus Ex, Rune, Kingpin, and others automatically bypass `\secure\` challenges to accept dedicated servers without dropped connections.
6. **Zero Outbound Probing (100% Passive)**:
   * Master server issues **zero** unsolicited outbound UDP query packets to the internet.
   * Eliminates firewall triggers, connection slot exhaustion, and DDoS false positives on community game hosts.
7. **Anti-Abuse & Public Hardening**:
   * **Zero-Load Snapshot Cache**: The public directory reads from an in-memory atomic pointer with zero lock contention.
   * **Zero-XSS Sanitization**: Quake color codes (`^1`, `^7`) are automatically stripped, strings are length-capped, and HTML entities are escaped.
   * **IP Blacklisting & Private IP Filtering**: Unroutable RFC1918 addresses and blacklisted IPs are rejected at the edge.

---

## Ports & Protocols

| Port | Protocol | Purpose / Games |
| :--- | :--- | :--- |
| **`28900`** | TCP | GameSpy v0 master list query (`\list\`) + 333Networks federation (`\sync\`) |
| **`28902`** | TCP | **Unreal Tournament 2004 & 2003** in-game browser & uplink (`[IpDrv]`) |
| **`28910`** | TCP | **GameSpy v1** master list query (Halo, Battlefield 1942/2, Crysis) |
| **`27900`** | UDP | Dual GameSpy heartbeats (`\heartbeat\`) + Quake 2 in-game browser (`query\n`) |
| **`27950`** | UDP | Quake 3 Arena / idTech 3 / OpenArena / KingpinQ3 (`getservers`) |
| **`20510`** | UDP | Call of Duty 1 client browser |
| **`20610`** | UDP | Call of Duty: United Offensive client browser |
| **`20710`** | UDP | Call of Duty 2 client browser |
| **`20810`** | UDP | Call of Duty 4: Modern Warfare client browser |
| **`28060`** | UDP | Star Wars Jedi Knight II: Jedi Outcast |
| **`29060`** | UDP | Star Wars Jedi Knight: Jedi Academy |
| **`20110`** | UDP | Soldier of Fortune II: Double Helix |
| **`27953`** | UDP | Star Trek: Voyager - Elite Force |
| **`29250`** | UDP | Star Trek: Elite Force II |
| **`27777`** | UDP | DarkPlaces / Xonotic / Nexuiz |
| **`8085`** | TCP | Internal REST API (communicates between Go daemon and NGINX) |
| **`80 / 3000`** | TCP | Public NGINX live server directory & master setup guide |

---

## Quickstart: Docker Deployment

Pre-built multi-architecture container images are published to the GitHub Container Registry (`ghcr.io`).

### Option 1: Single `docker run` Command

Run the unified all-in-one container directly:

```bash
docker run -d \
  --name openmaster \
  -p 28900:28900/tcp \
  -p 28902:28902/tcp \
  -p 28910:28910/tcp \
  -p 27900:27900/udp \
  -p 27950:27950/udp \
  -p 20510:20510/udp \
  -p 20610:20610/udp \
  -p 20710:20710/udp \
  -p 20810:20810/udp \
  -p 28060:28060/udp \
  -p 29060:29060/udp \
  -p 20110:20110/udp \
  -p 27953:27953/udp \
  -p 29250:29250/udp \
  -p 27777:27777/udp \
  -p 80:80 \
  --restart unless-stopped \
  ghcr.io/shavargo/openmaster:latest
```

Open [http://localhost](http://localhost) in your browser.

### Option 2: Docker Compose

```bash
curl -sSL https://raw.githubusercontent.com/shavargo/openmaster/main/docker-compose.yml | docker compose -f - up -d
```

---

## Deployment from Cloned Repository

To start the services:
```bash
docker compose up -d
```

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

### 2. Preview the Web Directory
The web directory consists of static files in `web/public/` served via NGINX:
```bash
docker build -f Dockerfile.web -t openmaster-web .
docker run -p 80:80 openmaster-web
```
Or open [web/public/index.html](web/public/index.html) directly in any browser.

---

## Attribution & Acknowledgements

- Server address registries and multi-master mirrors are ingested from [retro-server-mirror](https://github.com/shavargo/retro-server-mirror).
- Game definitions and metadata originate from [lookout-games](https://github.com/molycode/lookout-games) (MIT, Copyright (c) 2026 moly).
- Master server protocols and network data from [OpenSpy](https://openspy.net/) and [333networks](https://333networks.com/).

---

## License

This project is licensed under the [MIT License](LICENSE) - see the [LICENSE](LICENSE) file for details.
