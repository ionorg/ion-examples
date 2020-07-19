Ion Examples
===
A work-in-progress suite of code use examples for [Ion Conference Server](https://github.com/pion/ion)

**SEEKING CODE REVIEWS** - hi, i (@leewardbound) am very new to `golang` and i likely made amateur mistakes in all of these examples! if you enjoy using these tools, please help make them better, and submit your feedback or improvements!

join-from-disk
===
join an ion room from a file on the disk; stable (used in `ion-e2e-test`)

Example:
> `cd join-from-disk && go run main.go -e wss://my.ion.server/ws -r test_room -v video.ivf`


join-from-screenshare
===
join an ion room sharing the entire screen; stable

Example:
> `cd join-from-disk && go run main.go -e wss://my.ion.server/ws -w 1080`

join-from-webcam
===
join an ion room sharing a webcam device; **work-in-progress** (please open issues if it doesn't run for you)

Example:
> `cd join-from-disk && go run main.go -e wss://my.ion.server/ws`
