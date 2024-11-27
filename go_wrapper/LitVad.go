package main

import (
	"encoding/json"
	"fmt"
	agoraservice "github.com/AgoraIO-Extensions/Agora-Golang-Server-SDK/v2/go_sdk/agoraservice"
	"os"
	"os/signal"
	"time"
)

type AudioLabel struct {
	BufferSize        int `json:"buffer_size"`
	SamplesPerChannel int `json:"samples_per_channel"`
	BytesPerSample    int `json:"bytes_per_sample"`
	Channels          int `json:"channels"`
	SampleRate        int `json:"sample_rate"`
	FarFieldFlag      int `json:"far_field_flag"`
	VoiceProb         int `json:"voice_prob"`
	Rms               int `json:"rms"`
	Pitch             int `json:"pitch"`
}

func AudioFrameToString(frame *agoraservice.AudioFrame) string {
	al := AudioLabel{
		BufferSize:        len(frame.Buffer),
		SamplesPerChannel: frame.SamplesPerChannel,
		BytesPerSample:    frame.BytesPerSample,
		Channels:          frame.Channels,
		SampleRate:        frame.SamplesPerSec,
		FarFieldFlag:      frame.FarFieldFlag,
		VoiceProb:         frame.VoiceProb,
		Rms:               frame.Rms,
		Pitch:             frame.Pitch,
	}
	alStr, _ := json.Marshal(al)
	return string(alStr)
}

func main() {

	bStop := new(bool)
	*bStop = false

	// catch ternimal signal
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		*bStop = true
		fmt.Println("Application terminated")
	}()

	userId := "111"
	svcCfg := agoraservice.NewAgoraServiceConfig()
	svcCfg.AppId = "77acef4ff99542f6a0014e5e63ce52de"
	agoraservice.Initialize(svcCfg)

	defer agoraservice.Release()
	mediaNodeFactory := agoraservice.NewMediaNodeFactory()
	defer mediaNodeFactory.Release()

	agoraservice.EnableExtension("agora.builtin", "agora_audio_label_generator", "", true)
	//agoraservice.GetAgoraParameter().SetParameters("{\"che.audio.label.enable\": true}")

	vad := agoraservice.NewAudioVadV2(
		&agoraservice.AudioVadConfigV2{
			PreStartRecognizeCount: 16,
			StartRecognizeCount:    30,
			StopRecognizeCount:     20,
			ActivePercent:          0.7,
			InactivePercent:        0.5,
			StartVoiceProb:         60,
			StartRms:               -60.0,
			StopVoiceProb:          70,
			StopRms:                -60.0,
		})
	defer vad.Release()

	senderCfg := agoraservice.RtcConnectionConfig{
		AutoSubscribeAudio: true,
		AutoSubscribeVideo: true,
		ClientRole:         agoraservice.ClientRoleBroadcaster,
		ChannelProfile:     agoraservice.ChannelProfileLiveBroadcasting,
	}
	conHandler := &agoraservice.RtcConnectionObserver{
		OnConnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Println("Connected")
		},
		OnDisconnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Println("Disconnected")
		},
		OnConnecting: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Printf("Connecting, reason %d\n", reason)
		},
		OnReconnecting: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Printf("Reconnecting, reason %d\n", reason)
		},
		OnReconnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Printf("Reconnected, reason %d\n", reason)
		},
		OnConnectionLost: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo) {
			fmt.Printf("Connection lost\n")
		},
		OnConnectionFailure: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, errCode int) {
			fmt.Printf("Connection failure, error code %d\n", errCode)
		},
		OnUserJoined: func(con *agoraservice.RtcConnection, uid string) {
			fmt.Println("user joined, " + uid)
		},
		OnUserLeft: func(con *agoraservice.RtcConnection, uid string, reason int) {
			fmt.Println("user left, " + uid)
		},
	}

	var preVadDump *os.File = nil
	var vadDump *os.File = nil
	defer func() {
		if preVadDump != nil {
			preVadDump.Close()
		}
		if vadDump != nil {
			vadDump.Close()
		}
	}()

	var vadCount *int = new(int)
	*vadCount = 0
	audioObserver := &agoraservice.AudioFrameObserver{
		OnPlaybackAudioFrameBeforeMixing: func(localUser *agoraservice.LocalUser, channelId string, uid string, frame *agoraservice.AudioFrame) bool {
			// do something
			// fmt.Printf("Playback audio frame before mixing, from userId %s\n", userId)
			if preVadDump == nil {
				var err error
				preVadDump, err = os.OpenFile(fmt.Sprintf("./pre_vad_%s_%v.pcm", userId, time.Now().Format("2006-01-02-15-04-05")), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
				if err != nil {
					fmt.Println("Failed to create dump file: ", err)
				}
			}
			if preVadDump != nil {
				fmt.Printf("PreVad: %s\n", AudioFrameToString(frame))
				preVadDump.Write(frame.Buffer)
			}
			// vad
			vadResult, state := vad.Process(frame)
			duration := 0
			if vadResult != nil {
				duration = vadResult.SamplesPerChannel / 16
			}
			fmt.Printf("qidebug, vad state: %v\n", state)
			if state == agoraservice.VadStateIsSpeeking || state == agoraservice.VadStateStartSpeeking {
				fmt.Printf("Vad result: state: %v, duration: %v\n", state, duration)
				if vadDump == nil {
					*vadCount++
					var err error
					vadDump, err = os.OpenFile(fmt.Sprintf("./vad_%d_%s_%v.pcm", *vadCount, userId, time.Now().Format("2006-01-02-15-04-05")), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
					if err != nil {
						fmt.Println("Failed to create dump file: ", err)
					}
				}
				if vadDump != nil {
					vadDump.Write(vadResult.Buffer)
				}
			} else {
				if vadDump != nil {
					vadDump.Close()
					vadDump = nil
				}
			}
			return true
		},
	}

	senderCon := agoraservice.NewRtcConnection(&senderCfg)
	defer senderCon.Release()

	localUser := senderCon.GetLocalUser()
	localUser.SetPlaybackAudioFrameBeforeMixingParameters(1, 16000)
	senderCon.RegisterObserver(conHandler)
	localUser.RegisterAudioFrameObserver(audioObserver)

	sender := mediaNodeFactory.NewAudioPcmDataSender()
	defer sender.Release()
	track := agoraservice.NewCustomAudioTrackPcm(sender)
	defer track.Release()

	localUser.SetAudioScenario(agoraservice.AudioScenarioChorus)
	senderCon.Connect("007eJxSYPh9+si7HeVhNyU6GrYGPPo0xdJjef0kTdMCZ8ZJ+cteNM9RYDA3T0xOTTNJS7O0NDUxSjNLNDAwNEk1TTUzTk41NUpJFUlxTW8IZGTYqVnDwMjAyMDCwMgA4jOBSWYwyQIm2RgKM0tSi0uYGQwNDQEBAAD///8FJKs=", "qitest", userId)

	track.SetEnabled(true)
	localUser.PublishAudio(track)

	track.AdjustPublishVolume(100)

	for !(*bStop) {
		time.Sleep(1 * time.Second)
	}

	localUser.UnpublishAudio(track)
	track.SetEnabled(false)
	senderCon.Disconnect()

}
