function uuidv4() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

export default class Client {

    constructor(app, config) {
        this.app = app
        this.config = config

        this.streams = {}

        this.socket = new WebSocket(config.endpoint)
        this.socket.addEventListener('message', this.onMessage.bind(this))

        this.pc = new RTCPeerConnection({
            iceServers: [{
                urls: config.stun,
            }],
        })

        this.registerPeerEvents()
    }


    async join() {
        const offer = await this.pc.createOffer()
        await this.pc.setLocalDescription(offer)
        const id = uuidv4()

        this.socket.send(JSON.stringify({
            id,
            method: 'join',
            params: {
                offer: this.pc.localDescription,
                sid: this.config.sid,
            },
        }))


        this.socket.addEventListener('message', (event) => {
            const resp = JSON.parse(event.data)
            if (resp.id === id) {
                this.app.logger.info(`Got publish answer`)

                // Hook this here so it's not called before joining
                this.pc.onnegotiationneeded = async() => {
                    this.app.logger.info('Renegotiating')
                    const offer = await this.pc.createOffer()

                    await this.pc.setLocalDescription(offer)
                    const id = uuidv4()
                    this.socket.send(JSON.stringify({
                        id,
                        method: 'offer',
                        params: { desc: offer },
                    }))

                    this.socket.addEventListener('message', (event) => {
                        const resp = JSON.parse(event.data)
                        if (resp.id === id) {
                            this.app.logger.info(`Got renegotiation answer`)
                            this.pc.setRemoteDescription(resp.result)
                        }
                    })
                }

                this.pc.setRemoteDescription(resp.result)
            }
        })
    }

    async onMessage(event) {
        const resp = JSON.parse(event.data)

        // Listen for server renegotiation notifications
        if (!resp.id && resp.method === 'offer') {
            this.app.logger.info(`Got offer notification`)
            await this.pc.setRemoteDescription(resp.params)
            const answer = await this.pc.createAnswer()
            await this.pc.setLocalDescription(answer)

            const id = uuidv4()
            this.app.logger.info(`Sending answer`)
            this.socket.send(JSON.stringify({
                id,
                method: 'answer',
                params: { desc: answer },
            }))
        }
    }


    registerPeerEvents() {
        this.pc.onconnectionstatechange = (event) => {
            console.log("CONNECTION STATE CHAnged", this.pc.connectionState)
        }

        this.pc.ontrack = ({ track, streams }) => {
            this.app.state.participants.push(track.id)
            this.app.logger.debug(`ontrack: ${track.id}`)


            track.onunmute = () => {
                if (track.kind === 'video') {
                    const trackStream = streams[0]
                    trackStream.addTrack(track)
                    this.streams[trackStream.id] = trackStream
                    this.app.state.streams.push({id: trackStream.id, type: 'remote'})
                }
            }

            // Local track ended.
            track.onended = () => {
                console.log("TRACK GONE")
            }

        }

        this.pc.onineconnectionstatechange = function() {
            if(this.pc.iceConnectionState == 'disconnected') {
                console.log('Disconnected');
            }
        }

        this.pc.onicecandidate = event => {
            if (event.candidate !== null) {
                this.socket.send(JSON.stringify({
                    method: 'trickle',
                    params: {
                        candidate: event.candidate,
                    },
                }))
            }
        }
    }


}