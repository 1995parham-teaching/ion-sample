// Parse URL parameters for configuration
const params = new URLSearchParams(window.location.search);
const sfuUrl = params.get('sfu') || 'ws://localhost:7000/ws';
const roomId = params.get('room') || 'test room';

const remotesDiv = document.getElementById("remotes");
const statusDiv = document.getElementById("status");

// Track streams and their video elements
const streamElements = new Map();

// Update status display
function updateStatus(message, isError = false) {
    if (statusDiv) {
        statusDiv.textContent = message;
        statusDiv.className = isError ? 'status error' : 'status';
    }
    console.log(isError ? `[ERROR] ${message}` : `[INFO] ${message}`);
}

var peerConnection = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    sdpSemantics: 'unified-plan',
});

updateStatus(`Connecting to ${sfuUrl}...`);

var webSocket = new WebSocket(sfuUrl);
var jrpc = new simple_jsonrpc();

// Send jrpc event over websocket
jrpc.toStream = (_msg) => { webSocket.send(_msg); };

webSocket.onerror = (error) => {
    updateStatus(`WebSocket error: ${error.message || 'Connection failed'}`, true);
};

webSocket.onclose = (event) => {
    updateStatus(`Connection closed (code: ${event.code}). Refresh to reconnect.`, true);
};

webSocket.onopen = async function () {
    updateStatus(`Connected! Joining room: ${roomId}`);

    peerConnection.oniceconnectionstatechange = () => {
        const state = peerConnection.iceConnectionState;
        console.log(`ICE connection state: ${state}`);
        if (state === 'connected') {
            updateStatus(`Streaming from room: ${roomId}`);
        } else if (state === 'disconnected' || state === 'failed') {
            updateStatus(`Connection ${state}. Try refreshing.`, true);
        }
    };

    peerConnection.onicecandidate = (event) => {
        if (event.candidate == null) {
            return;
        }

        jrpc.notification('trickle', {
            'candidate': event.candidate,
            'target': 0,
        });
    };

    // Add transceivers for both video and audio
    peerConnection.addTransceiver('video', { direction: 'recvonly' });
    peerConnection.addTransceiver('audio', { direction: 'recvonly' });

    peerConnection.ontrack = function (event) {
        const track = event.track;
        const stream = event.streams[0];

        if (!stream) {
            console.warn('Track received without stream');
            return;
        }

        console.log(`Received ${track.kind} track for stream ${stream.id}`);

        // Get or create container for this stream
        let container = streamElements.get(stream.id);
        if (!container) {
            container = document.createElement('div');
            container.className = 'stream-container';
            container.id = `stream-${stream.id}`;
            remotesDiv.appendChild(container);
            streamElements.set(stream.id, container);
        }

        if (track.kind === 'video') {
            let video = container.querySelector('video');
            if (!video) {
                video = document.createElement('video');
                video.srcObject = stream;
                video.autoplay = true;
                video.muted = true; // Muted for autoplay policy
                video.playsInline = true;
                container.appendChild(video);
            }
        } else if (track.kind === 'audio') {
            let audio = container.querySelector('audio');
            if (!audio) {
                audio = document.createElement('audio');
                audio.srcObject = stream;
                audio.autoplay = true;
                container.appendChild(audio);
            }
        }

        // Clean up when stream ends
        stream.onremovetrack = () => {
            if (stream.getTracks().length === 0) {
                const container = streamElements.get(stream.id);
                if (container && container.parentNode) {
                    container.parentNode.removeChild(container);
                }
                streamElements.delete(stream.id);
            }
        };
    };

    let offer = await peerConnection.createOffer();
    await peerConnection.setLocalDescription(offer);

    jrpc.call('join', {
        'offer': peerConnection.localDescription,
        'sid': roomId,
    });
};

webSocket.onmessage = (message) => {
    let data = JSON.parse(message.data);
    console.log('Received:', data);

    if (data.result && data.id) {
        // Response to our join request
        peerConnection.setRemoteDescription(data.result)
            .catch((e) => console.error('Error setting remote description:', e));
    } else if (data.method === 'offer') {
        // New offer from SFU (renegotiation)
        peerConnection.setRemoteDescription(data.params)
            .then(() => peerConnection.createAnswer())
            .then((answer) => peerConnection.setLocalDescription(answer))
            .then(() => {
                jrpc.call('answer', peerConnection.localDescription);
            })
            .catch((e) => console.error('Error handling offer:', e));
    } else if (data.method === 'trickle') {
        // ICE candidate from SFU
        peerConnection.addIceCandidate(data.params)
            .catch((e) => console.error('Error adding ICE candidate:', e));
    }
};
