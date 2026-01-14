// LiveKit Viewer
const params = new URLSearchParams(window.location.search);
const wsUrl = params.get('host') || 'ws://localhost:7880';
const roomName = params.get('room') || 'test-room';
const identity = params.get('identity') || 'viewer-' + Math.random().toString(36).substring(7);
const apiKey = params.get('api_key') || 'devkey';
const apiSecret = params.get('api_secret') || 'secret';

const remotesDiv = document.getElementById("remotes");
const statusDiv = document.getElementById("status");

function setStatus(msg, type) {
    statusDiv.textContent = msg;
    statusDiv.className = 'status' + (type ? ' ' + type : '');
    console.log(`[${type || 'info'}] ${msg}`);
}

// Base64url encode for JWT
const b64 = (s) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

// Generate JWT token (dev only - don't expose secrets in production)
async function makeToken() {
    const now = Math.floor(Date.now() / 1000);
    const jti = crypto.randomUUID();
    const header = b64(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
    const payload = b64(JSON.stringify({
        iss: apiKey,
        sub: identity,
        jti: jti,
        iat: now,
        exp: now + 86400,
        nbf: now,
        video: { room: roomName, roomJoin: true, canSubscribe: true, canPublish: false }
    }));
    const data = `${header}.${payload}`;
    const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(apiSecret),
        { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
    const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(data));
    return `${data}.${b64(String.fromCharCode(...new Uint8Array(sig)))}`;
}

function handleTrackSubscribed(track, publication, participant) {
    console.log(`Track subscribed: ${track.kind} from ${participant.identity}`);

    const containerId = `participant-${participant.identity}`;
    let container = document.getElementById(containerId);

    if (!container) {
        container = document.createElement('div');
        container.id = containerId;
        container.className = 'participant-container';

        const nameTag = document.createElement('div');
        nameTag.className = 'participant-name';
        nameTag.textContent = participant.identity;
        container.appendChild(nameTag);

        remotesDiv.appendChild(container);
    }

    const element = track.attach();
    element.id = `track-${publication.trackSid}`;
    container.appendChild(element);

    setStatus(`Receiving stream from: ${participant.identity}`, 'connected');
}

function handleTrackUnsubscribed(track, publication, participant) {
    console.log(`Track unsubscribed: ${track.kind} from ${participant.identity}`);
    track.detach().forEach(el => el.remove());

    const container = document.getElementById(`participant-${participant.identity}`);
    if (container && container.querySelectorAll('video, audio').length === 0) {
        container.remove();
    }
}

(async () => {
    try {
        setStatus('Generating token...');
        const token = await makeToken();

        setStatus(`Connecting to ${wsUrl}...`);
        const room = new LivekitClient.Room({
            adaptiveStream: true,
            dynacast: true,
        });

        room.on(LivekitClient.RoomEvent.TrackSubscribed, handleTrackSubscribed);
        room.on(LivekitClient.RoomEvent.TrackUnsubscribed, handleTrackUnsubscribed);
        room.on(LivekitClient.RoomEvent.Connected, () => setStatus(`Connected to: ${roomName}`, 'connected'));
        room.on(LivekitClient.RoomEvent.Disconnected, (reason) => setStatus(`Disconnected: ${reason || 'unknown'}`, 'error'));

        room.on(LivekitClient.RoomEvent.ParticipantConnected, (p) => console.log(`Participant joined: ${p.identity}`));
        room.on(LivekitClient.RoomEvent.ParticipantDisconnected, (p) => {
            console.log(`Participant left: ${p.identity}`);
            const container = document.getElementById(`participant-${p.identity}`);
            if (container) container.remove();
        });

        await room.connect(wsUrl, token);

        // Handle existing participants
        room.remoteParticipants.forEach((participant) => {
            participant.trackPublications.forEach((publication) => {
                if (publication.track && publication.isSubscribed) {
                    handleTrackSubscribed(publication.track, publication, participant);
                }
            });
        });
    } catch (e) {
        setStatus(`Error: ${e.message}`, 'error');
        console.error('Connection error:', e);
    }
})();
