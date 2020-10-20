export default (app) => {
    return {
        data: function() {
            return {
                app,
                talking: true,
            }
        },
        methods: {
            disconnect() {
                app.client.pc.close()
            },
            toggleTalk() {
                this.talking = !this.talking
                app.client.localStream.getTracks().forEach((track) => {
                    track.enabled = !track.enabled
                })
            },
        },
        mounted: async function() {
            const localStream = await navigator.mediaDevices.getUserMedia({
                audio: true,
                video: true,
            })

            app.client.streams[localStream.id] = localStream
            app.state.streams.push({id: localStream.id, type: 'local'})
            this.localStream = localStream.id

            localStream.getTracks().forEach((track) => {
                app.client.pc.addTrack(track, localStream)
                track.enabled = this.talking
            });
            app.client.join()
        },
        store: ['localStream', 'streams']
    }
}