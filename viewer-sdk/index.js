// Parse URL parameters for configuration
const params = new URLSearchParams(window.location.search);
const sfuUrl = params.get('sfu') || 'ws://localhost:7000/ws';
const roomId = params.get('room') || 'test room';

const remotesDiv = document.getElementById("remotes");
const statusDiv = document.getElementById("status");

// Update status display
function updateStatus(message, isError = false) {
    if (statusDiv) {
        statusDiv.textContent = message;
        statusDiv.className = isError ? 'status error' : 'status';
    }
    console.log(isError ? `[ERROR] ${message}` : `[INFO] ${message}`);
}

const config = {
    codec: 'vp8',
    iceServers: [
        {
            "urls": "stun:stun.l.google.com:19302",
        },
    ]
};

updateStatus(`Connecting to ${sfuUrl}...`);

const signalLocal = new Signal.IonSFUJSONRPCSignal(sfuUrl);
const clientLocal = new IonSDK.Client(signalLocal, config);

signalLocal.onopen = () => {
    updateStatus(`Connected! Joining room: ${roomId}`);
    clientLocal.join(roomId);
};

signalLocal.onerror = (error) => {
    updateStatus(`Connection error: ${error.message || 'Unknown error'}`, true);
};

signalLocal.onclose = () => {
    updateStatus('Connection closed. Refresh to reconnect.', true);
};

clientLocal.ontrack = (track, stream) => {
    console.log("got track", track.id, "for stream", stream.id, "kind:", track.kind);

    if (track.kind === "video") {
        track.onunmute = () => {
            // Check if we already have an element for this stream
            let remoteVideo = document.getElementById(`video-${stream.id}`);
            if (!remoteVideo) {
                remoteVideo = document.createElement("video");
                remoteVideo.id = `video-${stream.id}`;
                remoteVideo.srcObject = stream;
                remoteVideo.autoplay = true;
                remoteVideo.muted = true; // Muted for autoplay policy, unmute manually
                remoteVideo.playsInline = true;
                remotesDiv.appendChild(remoteVideo);
                updateStatus(`Receiving stream from room: ${roomId}`);
            }

            track.onremovetrack = () => {
                if (remoteVideo && remoteVideo.parentNode) {
                    remotesDiv.removeChild(remoteVideo);
                }
            };
        };
    } else if (track.kind === "audio") {
        // Handle audio tracks
        track.onunmute = () => {
            let remoteAudio = document.getElementById(`audio-${stream.id}`);
            if (!remoteAudio) {
                remoteAudio = document.createElement("audio");
                remoteAudio.id = `audio-${stream.id}`;
                remoteAudio.srcObject = stream;
                remoteAudio.autoplay = true;
                remotesDiv.appendChild(remoteAudio);
            }

            track.onremovetrack = () => {
                if (remoteAudio && remoteAudio.parentNode) {
                    remotesDiv.removeChild(remoteAudio);
                }
            };
        };
    }
};
