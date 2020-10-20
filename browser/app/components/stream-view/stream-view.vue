<component class="c-stream-view widget">
    <div class="options">
        <button @click="toggleTalk">
            <span v-if="talking">Talking</span>
            <span v-else>Muted</span>
        </button>
        <button @click="disconnect">
            Disconnect
        </button>
    </div>
    <video
        v-for="stream of streams"
        :srcObject.prop="app.client.streams[stream.id]" autoplay="true"
        :muted="stream.type === 'local'"
    />
</component>