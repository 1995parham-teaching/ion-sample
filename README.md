# LiveKit Sample

A teaching example demonstrating WebRTC media broadcasting using [LiveKit](https://livekit.io/)
(Selective Forwarding Unit) with [Pion](https://github.com/pion) libraries.

## Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                             LiveKit Server                                  │
│                        (Selective Forwarding Unit)                          │
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │   Room Manager  │    │  Media Router   │    │ Signaling (WS)  │          │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘          │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                    WebSocket + Protobuf
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
        ▼                          ▼                          ▼
┌───────────────┐          ┌───────────────┐          ┌───────────────┐
│  Go Client    │          │  JS Client    │          │  JS Client    │
│  (Publisher)  │          │  (Viewer 1)   │          │  (Viewer 2)   │
│               │          │               │          │               │
│ ┌───────────┐ │          │ ┌───────────┐ │          │ ┌───────────┐ │
│ │  Camera   │ │          │ │  Video    │ │          │ │  Video    │ │
│ │  + Mic    │ │          │ │  Player   │ │          │ │  Player   │ │
│ └───────────┘ │          │ └───────────┘ │          │ └───────────┘ │
└───────────────┘          └───────────────┘          └───────────────┘
     Send Only                Receive Only               Receive Only
```

### Components

| Component    | Language   | Role      | Description                                                   |
| ------------ | ---------- | --------- | ------------------------------------------------------------- |
| `publisher/` | Go         | Publisher | Captures camera/microphone, encodes to VP8, broadcasts to SFU |
| `viewer/`    | JavaScript | Viewer    | Receives streams using LiveKit client SDK                     |

## Project Structure

```text
ion-sample/
├── publisher/
│   ├── main.go          # Go broadcaster client
│   ├── go.mod           # Go module definition
│   └── go.sum           # Dependency checksums
├── viewer/
│   ├── index.html       # HTML page
│   └── index.js         # LiveKit SDK implementation
├── docker-compose.yml   # LiveKit server
├── justfile             # Task runner commands
├── LICENSE
└── README.md
```

## Configuration

### Go Client (`publisher/main.go`)

| Flag           | Default                 | Description           |
| -------------- | ----------------------- | --------------------- |
| `-host`        | `http://localhost:7880` | LiveKit server URL    |
| `-api-key`     | `devkey`                | LiveKit API key       |
| `-api-secret`  | `secret`                | LiveKit API secret    |
| `-room`        | `test-room`             | Room name             |
| `-identity`    | `go-publisher`          | Participant identity  |

| Setting      | Value                     | Description             |
| ------------ | ------------------------- | ----------------------- |
| Video Codec  | VP8                       | Video encoding format   |
| Bitrate      | 500 kbps                  | Target video bitrate    |
| Resolution   | 640x480                   | Video dimensions        |
| Frame Format | YUY2                      | Raw video format        |

### JavaScript Client (`viewer/`)

Configuration via URL parameters:

| Parameter    | Default                 | Description          |
| ------------ | ----------------------- | -------------------- |
| `host`       | `ws://localhost:7880`   | LiveKit WebSocket URL|
| `room`       | `test-room`             | Room name            |
| `identity`   | `viewer-<random>`       | Participant identity |
| `api_key`    | `devkey`                | LiveKit API key      |
| `api_secret` | `secret`                | LiveKit API secret   |

Example: `index.html?room=my-room&identity=viewer1`

> **Note:** The viewer generates JWT tokens client-side for development convenience.
> In production, tokens should be generated server-side.

## Dependencies

### Go

| Package                      | Purpose                  |
| ---------------------------- | ------------------------ |
| `livekit/protocol`           | LiveKit protocol types   |
| `livekit/server-sdk-go/v2`   | LiveKit server SDK       |
| `pion/webrtc/v4`             | WebRTC implementation    |
| `pion/mediadevices`          | Camera/microphone access |

### JavaScript

| Library          | Version | Purpose              |
| ---------------- | ------- | -------------------- |
| `livekit-client` | 2.9.1   | LiveKit client SDK   |

## Usage

### Prerequisites

- Docker and Docker Compose (for LiveKit server)
- Go 1.23+ (for publisher)
- Camera and microphone (for publisher)
- Modern web browser (for viewer)
- [just](https://github.com/casey/just) command runner (optional)

### Quick Start

```bash
# Start LiveKit server
just sfu

# In another terminal, start the viewer
just serve
# Open http://localhost:8080 in your browser

# In another terminal, start the publisher
just publish
```

### Using justfile

```bash
# Start LiveKit server
just sfu

# Stop LiveKit server
just sfu-down

# View LiveKit logs
just sfu-logs

# Run publisher
just publish

# Run publisher with custom settings
just publish host="http://192.168.1.100:7880"

# Serve viewer
just serve

# Serve viewer on custom port
just serve port=3000

# Build publisher binary
just build

# Update Go dependencies
just update-deps
```

### Manual Usage

**Start LiveKit Server:**

```bash
docker compose up -d
```

**Publisher:**

```bash
cd publisher
go run main.go -host http://localhost:7880 -room my-room
```

**Viewer:**

```bash
cd viewer
python3 -m http.server 8080
# Open http://localhost:8080?room=my-room
```

## Authentication

LiveKit uses JWT tokens for authentication. The token contains:

- **API Key**: Identifies the application
- **API Secret**: Signs the token (keep secret!)
- **Video Grant**: Permissions (room join, publish, subscribe)
- **Identity**: Unique participant identifier

### Token Structure

```json
{
  "iss": "devkey",
  "sub": "participant-identity",
  "iat": 1234567890,
  "exp": 1234654290,
  "video": {
    "room": "test-room",
    "roomJoin": true,
    "canSubscribe": true,
    "canPublish": true
  }
}
```

## References

- [LiveKit](https://livekit.io/) - Open source WebRTC SFU
- [LiveKit Docs](https://docs.livekit.io/) - Official documentation
- [Pion WebRTC](https://github.com/pion/webrtc) - Pure Go WebRTC implementation
- [Pion MediaDevices](https://github.com/pion/mediadevices) - Media device access for Go
- [WebRTC API](https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API) - MDN documentation

## License

See [LICENSE](LICENSE) file.
