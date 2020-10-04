// Package join-from-disk contains example for joining an ion instance
// and publishing a video feed from a file on the disk
// use `ffmpeg -i {input_file} -g 30 {output_name}.ivf`
// to generate an ivf video file
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cloudwebrtc/go-protoo/client"
	"github.com/cloudwebrtc/go-protoo/logger"
	"github.com/cloudwebrtc/go-protoo/peer"
	"github.com/cloudwebrtc/go-protoo/transport"
	"github.com/google/uuid"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfreader"
)

var (
	endpoint string
	video    string
	rid      string
	username string
	duration int
)

// AnswerJSEP is part of Answer JSON reply
type AnswerJSEP struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

// Answer is a JSON reply
type Answer struct {
	JSEP AnswerJSEP `json:"jsep"`
	mid  string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "join",
		Short: "Join a room and publish a test feed",
		Run: func(cmd *cobra.Command, args []string) {
			doJoin()
		},
	}

	rootCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "ws://localhost:9090", "ion biz server websocket endpoint")
	rootCmd.PersistentFlags().StringVarP(&rid, "ion_room", "r", "test", "room name to join")
	rootCmd.PersistentFlags().StringVarP(&video, "videofile", "f", "pink.video", "video file to join with")
	rootCmd.PersistentFlags().StringVarP(&username, "ion_username", "u", "", "username to join with")
	rootCmd.PersistentFlags().IntVarP(&duration, "duration", "d", 60, "duration to join for (seconds) or 0 means forever")
	viper.AutomaticEnv()

	viper.BindPFlags(rootCmd.PersistentFlags())
	endpoint = viper.GetString("endpoint")
	rid = viper.GetString("ion_room")
	video = viper.GetString("videofile")
	username = viper.GetString("ion_username")
	if username == "" {
		username = video
	}
	duration = viper.GetInt("duration")

	rootCmd.Execute()
}

func doJoin() {
	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()

	// Create a new RTCPeerConnection
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:       []string{"turn:turn.streamhuddle.com:3472"},
				Username:   "huddle",
				Credential: "huddle999",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	offer, _ := peerConnection.CreateOffer(nil)

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	peerConnection.SetLocalDescription(offer)
	<-gatherComplete

	// Create a video track
	videoTrack, addTrackErr := peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeVideo, "VP8"), rand.Uint32(), "video", "pion")
	if addTrackErr != nil {
		panic(addTrackErr)
	}
	if _, addTrackErr = peerConnection.AddTrack(videoTrack); err != nil {
		panic(addTrackErr)
	}

	startTime := time.Now()

	go func() {
		// Open a IVF file and start reading using our IVFReader
		file, ivfErr := os.Open(video)
		if ivfErr != nil {
			panic(ivfErr)
		}

		ivf, header, ivfErr := ivfreader.NewWith(file)
		if ivfErr != nil {
			panic(ivfErr)
		}

		// Wait for connection established
		<-iceConnectedCtx.Done()
		<-gatherComplete

		// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
		// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
		sleepTime := time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000)
		for {
			frame, _, ivfErr := ivf.ParseNextFrame()
			if ivfErr == io.EOF {
				logger.Debugf("restarting video...")
				file, ivfErr = os.Open(video)
				if ivfErr != nil {
					panic(ivfErr)
				}

				ivf, header, ivfErr = ivfreader.NewWith(file)
				if ivfErr != nil {
					panic(ivfErr)
				}
			}

			if ivfErr != nil {
				panic(ivfErr)
			}

			time.Sleep(sleepTime)
			if ivfErr = videoTrack.WriteSample(media.Sample{Data: frame, Samples: 90000}); ivfErr != nil {
				panic(ivfErr)
			}

			elapsed := time.Now().Sub(startTime).Seconds()
			if duration > 0 && int(elapsed) > duration {
				logger.Infof("%d seconds elapsed, all done!", duration)
				os.Exit(0)
			}
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		logger.Infof("Connection State has changed %s", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	peerID := uuid.New().String()

	client.NewClient(endpoint+"?peer="+peerID, func(con *transport.WebSocketTransport) {
		logger.Infof("Connected to biz server => %s", endpoint)

		pr := peer.NewPeer(peerID, con)

		handleRequest := func(request peer.Request, accept peer.RespondFunc, reject peer.RejectFunc) {
			method := request.Method
			logger.Infof("handleRequest =>  (%s) ", method)
			if method == "kick" {
				reject(486, "Busy Here")
			} else {
				accept(nil)
			}
		}

		handleNotification := func(notification peer.Notification) {
			logger.Infof("handleNotification => %s", notification.Method)
		}

		handleClose := func(err transport.TransportErr) {
			logger.Infof("handleClose => peer (%s) [%d] %s", pr.ID(), err.Code, err.Text)
		}

		go func() {
			for {
				select {
				case msg := <-pr.OnNotification:
					handleNotification(msg)
				case msg := <-pr.OnRequest:
					handleRequest(msg.Request, msg.Accept, msg.Reject)
				case msg := <-pr.OnClose:
					handleClose(msg)
				}
			}
		}()

		pr.Request("join", json.RawMessage(`{"rid":"`+rid+`","info":{"name":"`+username+`"}}`),
			func(result json.RawMessage) {
				logger.Infof("join %s success\n", rid)

				offer, err := peerConnection.CreateOffer(nil)
				if err != nil {
					panic(err)
				}

				publishInfo := map[string]interface{}{
					"rid": rid,
					"jsep": map[string]interface{}{
						"sdp":  string(offer.SDP),
						"type": "offer",
					},
					"options": map[string]interface{}{
						"codec":     "VP8",
						"bandwidth": 1024,
					},
				}

				publish, err := json.Marshal(publishInfo)

				logger.Infof("Publish\n")
				if err != nil {
					logger.Infof("publish error", publish, err)
				}

				pr.Request("publish", publishInfo,
					func(result json.RawMessage) {
						logger.Infof("publish success!")

						if duration > 0 {
							logger.Infof("Looping video for %d seconds", duration)
						} else {
							logger.Infof("Looping video forever")
						}

						var answer Answer
						json.Unmarshal(result, &answer)
						peerConnection.SetRemoteDescription(webrtc.SessionDescription{
							Type: webrtc.SDPTypeAnswer,
							SDP:  answer.JSEP.SDP,
						})
						<-gatherComplete
					},
					func(code int, err string) {
						logger.Infof("publish reject: %d => %s", code, err)
					})

			},
			func(code int, err string) {
				logger.Infof("login reject: %d => %s", code, err)
			})

	},
	)

	for {
		// wait until end of file and exit
	}
}

// Search for Codec PayloadType
//
// Since we are answering we need to match the remote PayloadType
func getPayloadType(m webrtc.MediaEngine, codecType webrtc.RTPCodecType, codecName string) uint8 {
	for _, codec := range m.GetCodecsByKind(codecType) {
		if codec.Name == codecName {
			return codec.PayloadType
		}
	}
	panic(fmt.Sprintf("Remote peer does not support %s", codecName))
}
