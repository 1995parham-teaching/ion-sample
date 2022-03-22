var peerConnection = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    sdpSemantics: 'unified-plan',
})

var webSocket = new WebSocket("ws://ion-sfu-mohammad-sandbox.apps.private.okd4.teh-1.snappcloud.io/ws");

var jrpc = new simple_jsonrpc();

// send jrpc event over websocket
jrpc.toStream = (_msg) => { webSocket.send(_msg); };

webSocket.onerror = (error) => { console.log("an error has been occured:", error) };

webSocket.onclose = (event) => { console.log("websocket connection has been closed:", event) };

webSocket.onopen = async function () {
    peerConnection.oniceconnectionstatechange = (event) => { console.log(event) }

    peerConnection.onicecandidate = (candidate) => {
        if (candidate.candidate == null) {
            return
        }

        jrpc.notification('trickle', {
            'candidate': candidate.candidate,
            'target': 0,
        })
    }

    peerConnection.addTransceiver('video')

    peerConnection.ontrack = function (event) {
        document.getElementById("remotes").srcObject = event.streams[0]
    }

    let offer = await peerConnection.createOffer()
    await peerConnection.setLocalDescription(offer)

    jrpc.call('join', {
        'offer': peerConnection.localDescription,
        'sid': "test room",
    })
};

webSocket.onmessage = (message) => {
    let data = JSON.parse(message.data)
    console.log(data)

    if (data.id != null && data.id == 1) {
        peerConnection.setRemoteDescription(data.result)
    } else if (data.method == 'offer') {
        peerConnection.setRemoteDescription(data.params).catch((e) => console.log(e))
        jrpc.call('answer', peerConnection.localDescription)
    } else if (data.method == 'trickle') {
        this.peerConnection.addIceCandidate(data.params).catch((e) => console.log(e))
    }
};
