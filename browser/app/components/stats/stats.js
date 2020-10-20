export default (app) => {
    return {
        mounted: async function() {
            const stats = await app.client.pc.getStats()
            console.log("STATS", stats)
        },
        store: ['client', 'streams'],
    }
}