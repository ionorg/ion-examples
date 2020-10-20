# Pub-from-disk

This example demonstrates how to send video and/or audio to an ion-sfu from
files on disk. The example expect an IVF file named `output.ivf` containing a
VP8 track and/or a `output.ogg` that contains a Opus track.

## Usage

```bash
cd ion-examples/ion-sfu/pub-from-disk
go build -o pub main.go
wget -c https://ia801602.us.archive.org/11/items/Rick_Astley_Never_Gonna_Give_You_Up/Rick_Astley_Never_Gonna_Give_You_Up.mp4 -O output.mp4
ffmpeg -i output.mp4 -g 30 output.ivf
ffmpeg -i output.mp4 -c:a libopus -page_duration 20000 -vn output.ogg
# Use pub <sfu-sid>
./pub icestream
```

Congrats, you are now publishing video to the ion-sfu! You should see the video
play in the browser.
