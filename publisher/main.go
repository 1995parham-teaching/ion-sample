package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v4"

	// Camera and microphone drivers
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
)

var (
	host      string
	apiKey    string
	apiSecret string
	room      string
	identity  string
)

func main() {
	flag.StringVar(&host, "host", "http://localhost:7880", "LiveKit server URL")
	flag.StringVar(&apiKey, "api-key", "devkey", "LiveKit API key")
	flag.StringVar(&apiSecret, "api-secret", "secret", "LiveKit API secret")
	flag.StringVar(&room, "room", "test-room", "Room to join")
	flag.StringVar(&identity, "identity", "go-publisher", "Participant identity")
	flag.Parse()

	log.Printf("Joining room %s as %s", room, identity)

	// Setup VP8 codec for video encoding
	vpxParams, err := vpx.NewVP8Params()
	if err != nil {
		log.Fatalf("Failed to create VP8 params: %v", err)
	}
	vpxParams.BitRate = 500_000 // 500kbps

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&vpxParams),
	)

	// Print available devices
	log.Println("Available devices:", mediadevices.EnumerateDevices())

	// Get camera stream
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

	tracks := stream.GetTracks()
	defer func() {
		for _, track := range tracks {
			track.Close()
		}
	}()

	// Connect to LiveKit room
	roomCallback := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackPublished: func(publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
				log.Printf("Track published: %s by %s", publication.SID(), participant.Identity())
			},
		},
		OnDisconnected: func() {
			log.Println("Disconnected from room")
		},
		OnReconnecting: func() {
			log.Println("Reconnecting to room...")
		},
		OnReconnected: func() {
			log.Println("Reconnected to room")
		},
	}

	log.Printf("Connecting to %s", host)
	lkRoom, err := lksdk.ConnectToRoom(host, lksdk.ConnectInfo{
		APIKey:              apiKey,
		APISecret:           apiSecret,
		RoomName:            room,
		ParticipantIdentity: identity,
	}, roomCallback)
	if err != nil {
		log.Fatalf("Failed to connect to room: %v", err)
	}
	defer lkRoom.Disconnect()

	log.Printf("Connected to room: %s as %s", room, identity)

	// Publish tracks
	for _, track := range tracks {
		log.Printf("Publishing track: %s (kind: %s)", track.ID(), track.Kind())

		var trackLocal webrtc.TrackLocal
		var ok bool
		if trackLocal, ok = track.(webrtc.TrackLocal); !ok {
			log.Printf("Track %s is not a TrackLocal, skipping", track.ID())
			continue
		}

		publication, err := lkRoom.LocalParticipant.PublishTrack(trackLocal, &lksdk.TrackPublicationOptions{
			Name:   track.ID(),
			Source: livekit.TrackSource_CAMERA,
		})
		if err != nil {
			log.Printf("Failed to publish track %s: %v", track.ID(), err)
			continue
		}

		log.Printf("Published track: %s (SID: %s)", track.ID(), publication.SID())
	}

	log.Println("Publishing... Press Ctrl+C to stop")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
