const state = {
    participants: [],
    streams: [],
    localStream: null
}
Object.assign(state, globalThis.env)

export default state