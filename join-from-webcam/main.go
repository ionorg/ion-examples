// Package join-from-webcam contains an example of joining an ion instance
// and publishing a stream from the local webcam
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwebrtc/go-protoo/client"
	"github.com/cloudwebrtc/go-protoo/logger"
	"github.com/cloudwebrtc/go-protoo/peer"
	"github.com/cloudwebrtc/go-protoo/transport"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/opus"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v2"
)

var (
	endpoint string
	rid      string
	username string
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
	rootCmd.PersistentFlags().StringVarP(&username, "ion_username", "u", "join-from-webcam", "username to join with")
	viper.AutomaticEnv()

	viper.BindPFlags(rootCmd.PersistentFlags())
	endpoint = viper.GetString("endpoint")
	rid = viper.GetString("ion_room")
	username = viper.GetString("ion_username")

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
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())
	if iceConnectedCtx != nil {
		// do nothing
	}
	md := mediadevices.NewMediaDevices(peerConnection)

	opusParams, err := opus.NewParams()
	if err != nil {
		panic(err)
	}
	opusParams.BitRate = 32000 // 32kbps

	vp8Params, err := vpx.NewVP8Params()
	if err != nil {
		panic(err)
	}
	vp8Params.BitRate = 100000 // 100kbps

	s, err := md.GetUserMedia(mediadevices.MediaStreamConstraints{
		Audio: func(c *mediadevices.MediaTrackConstraints) {
			c.Enabled = true
			c.AudioEncoderBuilders = []codec.AudioEncoderBuilder{&opusParams}
		},
		Video: func(c *mediadevices.MediaTrackConstraints) {
			c.FrameFormat = prop.FrameFormat(frame.FormatYUY2)
			c.Enabled = true
			c.Width = prop.Int(1280)
			c.Height = prop.Int(720)
			c.VideoEncoderBuilders = []codec.VideoEncoderBuilder{&vp8Params}
		},
	})
	if err != nil {
		panic(err)
	}

	for _, tracker := range s.GetTracks() {
		t := tracker.Track()
		tracker.OnEnded(func(err error) {
			fmt.Printf("Track (ID: %s, Label: %s) ended with error: %v\n",
				t.ID(), t.Label(), err)
		})
		_, err = peerConnection.AddTransceiverFromTrack(t,
			webrtc.RtpTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)
		if err != nil {
			panic(err)
		}
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
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

				logger.Infof("Publish Message: %s\n", publish)

				pr.Request("publish", publishInfo,
					func(result json.RawMessage) {
						logger.Infof("publish success!")
						var answer Answer
						json.Unmarshal(result, &answer)

						peerConnection.SetRemoteDescription(webrtc.SessionDescription{
							Type: webrtc.SDPTypeAnswer,
							SDP:  answer.JSEP.SDP,
						})
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
