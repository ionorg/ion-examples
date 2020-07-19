Ion Examples
===
A work-in-progress suite of code use examples for [Ion Conference Server](https://github.com/pion/ion)

**SEEKING CODE REVIEWS** - hi, i (@leewardbound) am very new to `golang` and i likely made amateur mistakes in all of these examples! if you enjoy using these tools, please help make them better, and submit your feedback or improvements!

join-from-disk
===
join an ion room from a file on the disk; stable (used in `ion-e2e-test`)
**DOCKER NOTE**: Has been observed to hang forever inside `ubuntu:20.04` docker containers, use `20.10` instead

Example:
> `cd join-from-disk && go run main.go -e wss://my.ion.server/ws -r test_room -v video.ivf`


join-from-screenshare
===
join an ion room sharing the entire screen; stable
**LINUX ONLY**: see  [pion/mediadevices](https://github.com/pion/mediadevices)

Example:
> `cd join-from-screenshare && go run main.go -e wss://my.ion.server/ws -w 1080`

join-from-webcam
===
join an ion room sharing a webcam device; **work-in-progress** (please open issues if it doesn't run for you)
**COMPATABILITY**: see  [pion/mediadevices](https://github.com/pion/mediadevices)

Example:
> `cd join-from-webcam && go run main.go -e wss://my.ion.server/ws`

Building / Development Notes
===
You can build a binary for any of these examples by running `go build .` in the directory; all my examples have so far only been tested on linux (Ubuntu 20.04)
