# Ion SFU Sample

A teaching example demonstrating WebRTC media broadcasting using [Ion SFU](https://github.com/pion/ion-sfu) (Selective Forwarding Unit) with [Pion](https://github.com/pion) libraries.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Ion SFU Server                                  │
│                         (Selective Forwarding Unit)                          │
│                                                                              │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │   Room Manager  │    │  Media Router   │    │ Signaling (WS)  │          │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                    WebSocket + JSON-RPC 2.0
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

| Component | Language | Role | Description |
|-----------|----------|------|-------------|
| `publisher/` | Go | Publisher | Captures camera/microphone, encodes to VP8, broadcasts to SFU |
| `viewer-sdk/` | JavaScript | Viewer | Receives streams using Ion SDK (high-level API) |
| `viewer-raw/` | JavaScript | Viewer | Receives streams using raw WebRTC (low-level API) |

## Signaling Protocol

Communication between clients and SFU uses **JSON-RPC 2.0 over WebSocket**.

### Message Flow

```
Publisher                         SFU                          Viewer
    │                              │                              │
    │──── join(offer, sid) ───────▶│                              │
    │◀─── answer ──────────────────│                              │
    │                              │                              │
    │◀───── trickle ──────────────▶│◀───── trickle ──────────────▶│
    │     (ICE candidates)         │     (ICE candidates)         │
    │                              │                              │
    │════ Media Stream (RTP) ═════▶│════ Media Stream (RTP) ═════▶│
    │                              │                              │
    │                              │──── offer ──────────────────▶│
    │                              │◀─── answer ──────────────────│
```

### JSON-RPC Methods

| Method | Direction | Description |
|--------|-----------|-------------|
| `join` | Client → SFU | Join a room with SDP offer |
| `offer` | SFU → Client | SFU sends offer for new peer connection |
| `answer` | Client → SFU | Client responds to SFU offer |
| `trickle` | Bidirectional | ICE candidate exchange |

### Message Examples

**Join Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 12345,
  "method": "join",
  "params": {
    "sid": "test room",
    "offer": {
      "type": "offer",
      "sdp": "v=0\r\no=- ..."
    }
  }
}
```

**Trickle (ICE Candidate):**
```json
{
  "jsonrpc": "2.0",
  "method": "trickle",
  "params": {
    "target": 0,
    "candidate": {
      "candidate": "candidate:...",
      "sdpMid": "0",
      "sdpMLineIndex": 0
    }
  }
}
```

## Project Structure

```
ion-sample/
├── publisher/
│   ├── main.go          # Go broadcaster client
│   ├── go.mod           # Go module definition
│   └── go.sum           # Dependency checksums
├── viewer-sdk/
│   ├── index.html       # HTML page with Ion SDK
│   └── index.js         # High-level Ion SDK implementation
├── viewer-raw/
│   ├── index.html       # HTML page with raw WebRTC
│   └── index.js         # Low-level WebRTC implementation
├── justfile             # Task runner commands
├── LICENSE
└── README.md
```

## Configuration

### Go Client (`publisher/main.go`)

| Setting | Value | Description |
|---------|-------|-------------|
| SFU Address | `localhost:7000` | WebSocket endpoint (configurable via `-addr` flag) |
| STUN Server | `stun.l.google.com:19302` | Google's public STUN server |
| Room ID | `test room` | Room identifier |
| Video Codec | VP8 | Video encoding format |
| Bitrate | 500 kbps | Target video bitrate |
| Resolution | 640x480 | Video dimensions |
| Frame Format | YUY2 | Raw video format |

### JavaScript Clients

Configuration is done via URL parameters:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `sfu` | `ws://localhost:7000/ws` | SFU WebSocket URL |
| `room` | `test room` | Room identifier |

Example: `index.html?sfu=ws://192.168.1.100:7000/ws&room=my-room`

## Dependencies

### Go

| Package | Version | Purpose |
|---------|---------|---------|
| `pion/webrtc/v3` | v3.1.24 | WebRTC implementation |
| `pion/mediadevices` | v0.3.2 | Camera/microphone access |
| `gorilla/websocket` | v1.5.0 | WebSocket client |
| `sourcegraph/jsonrpc2` | v0.1.0 | JSON-RPC 2.0 protocol |
| `google/uuid` | v1.3.0 | UUID generation |

### JavaScript (viewer-sdk/)

| Library | Version | Purpose |
|---------|---------|---------|
| `ion-sdk-js` | 1.8.1 | High-level Ion client SDK |

### JavaScript (viewer-raw/)

| Library | Purpose |
|---------|---------|
| `simple-jsonrpc-js` | JSON-RPC 2.0 implementation |

## Usage

### Prerequisites

- Go 1.17+
- Ion SFU server running
- Camera and microphone (for Go client)
- Modern web browser (for JS clients)
- [just](https://github.com/casey/just) command runner (optional)

### Using justfile

```bash
# Run publisher
just publish

# Run publisher with custom address
just publish 192.168.1.100:7000

# Serve viewer (Ion SDK version)
just serve-sdk

# Serve viewer (raw WebRTC version)
just serve-raw

# Update Go dependencies
just update-deps
```

### Manual Usage

**Publisher:**
```bash
cd publisher && go run main.go -addr localhost:7000
```

**Viewers:**
```bash
cd viewer-sdk && python3 -m http.server 8080
# Open http://localhost:8080?sfu=ws://localhost:7000/ws
```

## Code Walkthrough

### Go Client (`publisher/main.go`)

1. **WebSocket Connection** (line 78): Connects to SFU server
2. **WebRTC Configuration** (line 84-91): Sets up ICE servers and SDP semantics
3. **Media Capture** (line 122-133): Gets camera stream using `mediadevices.GetUserMedia()`
4. **Track Addition** (line 135-148): Adds media tracks with send-only direction
5. **Offer Creation** (line 151-160): Creates and sets local SDP offer
6. **ICE Handling** (line 167-194): Sends ICE candidates via `trickle` method
7. **Join Room** (line 217-232): Sends join request with offer
8. **Message Handler** (line 239-312): Processes SFU responses (answer, offer, trickle)

### JavaScript SDK Client (`viewer-sdk/index.js`)

1. **Signal Setup**: Creates JSON-RPC signal connection
2. **Client Creation**: Initializes Ion SDK client
3. **Join Room**: Joins room when connection opens
4. **Track Handler**: Creates video elements for incoming streams

### JavaScript Raw Client (`viewer-raw/index.js`)

1. **Peer Connection**: Creates RTCPeerConnection manually
2. **JSON-RPC Setup**: Configures JSON-RPC over WebSocket
3. **ICE Handling**: Sends candidates via `trickle`
4. **Transceiver**: Adds video transceiver for receiving
5. **Track Handler**: Displays received video
6. **Offer/Join**: Creates offer and joins room
7. **Message Handler**: Processes server responses

## References

- [Ion SFU](https://github.com/pion/ion-sfu) - Selective Forwarding Unit
- [Pion WebRTC](https://github.com/pion/webrtc) - Pure Go WebRTC implementation
- [Pion MediaDevices](https://github.com/pion/mediadevices) - Media device access for Go
- [Ion SDK JS](https://github.com/pion/ion-sdk-js) - JavaScript client SDK
- [WebRTC API](https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API) - MDN documentation

## License

See [LICENSE](LICENSE) file.
