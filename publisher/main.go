package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v4"
	"github.com/sourcegraph/jsonrpc2"

	// Note: If you don't have a camera or microphone or your adapters are not supported,
	//       you can always swap your adapters with our dummy adapters below.
	// _ "github.com/pion/mediadevices/pkg/driver/audiotest"
	// _ "github.com/pion/mediadevices/pkg/driver/videotest"
	// These are required to register camera and microphone adapter
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
)

// Candidate represents an ICE candidate to send to the SFU.
type Candidate struct {
	Target    int                     `json:"target"`
	Candidate webrtc.ICECandidateInit `json:"candidate"`
}

// ResponseCandidate represents an ICE candidate received from the SFU.
type ResponseCandidate struct {
	Target    int                      `json:"target"`
	Candidate *webrtc.ICECandidateInit `json:"candidate"`
}

// SDPDescription is a JSON-serializable SDP with string type.
type SDPDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// SendOffer is the object sent to join a room via the SFU.
type SendOffer struct {
	SID   string          `json:"sid"`
	Offer *SDPDescription `json:"offer"`
}

// TrickleResponse is received from the SFU server for ICE candidates.
type TrickleResponse struct {
	Params ResponseCandidate `json:"params"`
	Method string            `json:"method"`
}

// Response is received from the SFU over WebSocket.
type Response struct {
	Params *webrtc.SessionDescription `json:"params"`
	Result *webrtc.SessionDescription `json:"result"`
	Method string                     `json:"method"`
	Id     uint64                     `json:"id"`
}

// Publisher handles WebRTC publishing to Ion SFU.
type Publisher struct {
	peerConnection *webrtc.PeerConnection
	wsConn         *websocket.Conn
	wsMutex        sync.Mutex
	connectionID   uint64
	room           string
	done           chan struct{}
	closeOnce      sync.Once
	tracks         []mediadevices.Track
}

// toSDPDescription converts a webrtc.SessionDescription to a JSON-serializable format.
func toSDPDescription(sd *webrtc.SessionDescription) *SDPDescription {
	if sd == nil {
		return nil
	}
	return &SDPDescription{
		Type: sd.Type.String(),
		SDP:  sd.SDP,
	}
}

// writeMessage safely writes a message to the WebSocket connection.
func (p *Publisher) writeMessage(data []byte) error {
	p.wsMutex.Lock()
	defer p.wsMutex.Unlock()
	return p.wsConn.WriteMessage(websocket.TextMessage, data)
}

// sendJSONRPC sends a JSON-RPC request over WebSocket.
func (p *Publisher) sendJSONRPC(method string, params interface{}, id uint64) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	rawParams := json.RawMessage(paramsJSON)
	message := &jsonrpc2.Request{
		Method: method,
		Params: &rawParams,
	}

	if id != 0 {
		message.ID = jsonrpc2.ID{Num: id}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(message); err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	return p.writeMessage(buf.Bytes())
}

// handleMessages processes incoming WebSocket messages from the SFU.
func (p *Publisher) handleMessages() {
	defer close(p.done)

	for {
		_, message, err := p.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		var response Response
		if err := json.Unmarshal(message, &response); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			continue
		}

		switch {
		case response.Id == p.connectionID && response.Result != nil:
			// Response to our join request
			log.Println("Received SFU answer")
			if err := p.peerConnection.SetRemoteDescription(*response.Result); err != nil {
				log.Printf("Failed to set remote description: %v", err)
			}

		case response.Method == "offer" && response.Params != nil:
			// SFU sends an offer for renegotiation
			log.Println("Received renegotiation offer from SFU")
			if err := p.handleRenegotiation(response.Params); err != nil {
				log.Printf("Failed to handle renegotiation: %v", err)
			}

		case response.Method == "trickle":
			// SFU sends a new ICE candidate
			var trickleResponse TrickleResponse
			if err := json.Unmarshal(message, &trickleResponse); err != nil {
				log.Printf("Failed to unmarshal trickle response: %v", err)
				continue
			}

			if trickleResponse.Params.Candidate == nil {
				continue
			}

			log.Printf("Received ICE candidate: %s", trickleResponse.Params.Candidate.Candidate)
			if err := p.peerConnection.AddICECandidate(*trickleResponse.Params.Candidate); err != nil {
				log.Printf("Failed to add ICE candidate: %v", err)
			}
		}
	}
}

// handleRenegotiation processes an offer from the SFU and sends back an answer.
func (p *Publisher) handleRenegotiation(offer *webrtc.SessionDescription) error {
	if err := p.peerConnection.SetRemoteDescription(*offer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := p.peerConnection.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	if err := p.peerConnection.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	p.connectionID = uint64(uuid.New().ID())

	return p.sendJSONRPC("answer", toSDPDescription(p.peerConnection.LocalDescription()), p.connectionID)
}

// setupICEHandlers configures ICE candidate and connection state handlers.
func (p *Publisher) setupICEHandlers() {
	p.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		log.Printf("Sending ICE candidate: %s", candidate.Address)

		err := p.sendJSONRPC("trickle", &Candidate{
			Candidate: candidate.ToJSON(),
			Target:    0,
		}, 0)

		if err != nil {
			log.Printf("Failed to send ICE candidate: %v", err)
		}
	})

	p.peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE connection state: %s", state.String())

		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			log.Println("Connection lost, consider reconnecting...")
		}
	})

	p.peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer connection state: %s", state.String())
	})
}

// join sends a join request to the SFU with the local offer.
func (p *Publisher) join() error {
	offer, err := p.peerConnection.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := p.peerConnection.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	p.connectionID = uint64(uuid.New().ID())

	return p.sendJSONRPC("join", &SendOffer{
		Offer: toSDPDescription(p.peerConnection.LocalDescription()),
		SID:   p.room,
	}, p.connectionID)
}

// Close gracefully shuts down the publisher.
func (p *Publisher) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		// Stop media tracks first
		for _, track := range p.tracks {
			track.Close()
		}

		if p.peerConnection != nil {
			if err := p.peerConnection.Close(); err != nil {
				closeErr = err
			}
		}
		if p.wsConn != nil {
			if err := p.wsConn.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	})
	return closeErr
}

var (
	addr string
	room string
)

func main() {
	flag.StringVar(&addr, "addr", "localhost:7000", "SFU server address")
	flag.StringVar(&room, "room", "test room", "room to join")
	flag.Parse()

	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	log.Printf("Connecting to %s", u.String())

	wsConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("Failed to connect to SFU: %v", err)
	}

	publisher := &Publisher{
		wsConn: wsConn,
		room:   room,
		done:   make(chan struct{}),
	}
	defer publisher.Close()

	// Configure WebRTC
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	// Setup media engine with VP8 codec
	mediaEngine := webrtc.MediaEngine{}

	vpxParams, err := vpx.NewVP8Params()
	if err != nil {
		log.Fatalf("Failed to create VP8 params: %v", err)
	}
	vpxParams.BitRate = 500_000 // 500kbps

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&vpxParams),
	)
	codecSelector.Populate(&mediaEngine)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))
	publisher.peerConnection, err = api.NewPeerConnection(config)
	if err != nil {
		log.Fatalf("Failed to create peer connection: %v", err)
	}

	// Print available devices
	log.Println("Available devices:", mediadevices.EnumerateDevices())

	// Get user media (camera)
	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			c.FrameFormat = prop.FrameFormat(frame.FormatYUY2)
			c.Width = prop.Int(640)
			c.Height = prop.Int(480)
		},
		Codec: codecSelector,
	})
	if err != nil {
		log.Fatalf("Failed to get user media: %v", err)
	}

	// Add tracks to peer connection
	publisher.tracks = stream.GetTracks()
	for _, track := range publisher.tracks {
		track.OnEnded(func(err error) {
			if err != nil {
				log.Printf("Track (ID: %s) ended with error: %v", track.ID(), err)
			}
		})

		_, err = publisher.peerConnection.AddTransceiverFromTrack(track,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)
		if err != nil {
			log.Fatalf("Failed to add track: %v", err)
		}
		log.Printf("Added track: %s", track.ID())
	}

	// Setup handlers and join
	publisher.setupICEHandlers()

	go publisher.handleMessages()

	if err := publisher.join(); err != nil {
		log.Fatalf("Failed to join room: %v", err)
	}
	log.Printf("Joining room: %s", room)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-publisher.done:
		log.Println("Connection closed")
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
	}

	// Cleanup is handled by defer publisher.Close()
	log.Println("Shutdown complete")
}
