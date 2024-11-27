package main

import (
	"fmt"
	agoraservice "github.com/AgoraIO-Extensions/Agora-Golang-Server-SDK/v2/go_sdk/agoraservice"
	"os"
	"sync"
	"time"
)

func main() {

	bStop := new(bool)
	*bStop = false

	userId := "222"

	svcCfg := agoraservice.NewAgoraServiceConfig()
	svcCfg.AppId = "20338919f2ca4af4b1d7ec23d8870b56"
	agoraservice.Initialize(svcCfg)

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

	senderCon := agoraservice.NewConnection(&senderCfg)
	defer senderCon.Release()
	sender := senderCon.NewPcmSender()
	defer sender.Release()
	senderCon.Connect("", "qitest", "111")
	sender.Start()
	var stopSend *bool = new(bool)
	*stopSend = false
	waitSenderStop := &sync.WaitGroup{}
	waitSenderStop.Add(1)
	go func() {

		frame := agoraservice.PcmAudioFrame{
			Data:              make([]byte, 960),
			Timestamp:         0,
			SamplesPerChannel: 480,
			BytesPerSample:    2,
			NumberOfChannels:  1,
			SampleRate:        48000,
		}

		defer waitSenderStop.Done()
		file, err := os.Open("./test_data/lit_test_pcm.pcm")
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		sender.AdjustVolume(100)
		//sender.SetSendBufferSize(1000)

		sendCount := 0
		// send 180ms audio data
		for i := 0; i < 18; i++ {
			dataLen, err := file.Read(frame.Data)
			if err != nil || dataLen < 960 {
				fmt.Println("Finished reading file:", err)
				break
			}
			sendCount++
			sender.SendPcmData(&frame)
			//ret := sender.SendPcmData(&frame)
			//fmt.Printf("SendPcmData222 %d ret: %d\n", sendCount, ret)
		}

		// ffmpeg -i lit_test.m4a -ac 1 -ar 48000 -f s16le lit_test_pcm.pcm

		firstSendTime := time.Now()
		for !(*bStop) {
			shouldSendCount := int(time.Since(firstSendTime).Milliseconds()/10) - (sendCount - 18)
			fmt.Printf("qidebug, shouldSendCount %d \n", shouldSendCount)

			for i := 0; i < shouldSendCount; i++ {
				dataLen, err := file.Read(frame.Data)
				if err != nil || dataLen < 960 {
					fmt.Println("Finished reading file:", err)
					file.Seek(0, 0)
					i--
					continue
				}

				sendCount++
				sender.SendPcmData(&frame)
				//ret := sender.SendPcmData(&frame)
				//fmt.Printf("SendPcmData %d ret: %d\n", sendCount, ret)
			}
			fmt.Printf("Sent %d frames this time\n", shouldSendCount)
			time.Sleep(50 * time.Millisecond)
		}
		sender.Stop()
	}()

	//vad := agoraservice.NewAudioVad(&agoraservice.AudioVadConfig{
	//	StartRecognizeCount:    10,
	//	StopRecognizeCount:     6,
	//	PreStartRecognizeCount: 10,
	//	ActivePercent:          0.6,
	//	InactivePercent:        0.2,
	//})
	vad := agoraservice.NewAudioVad(&agoraservice.AudioVadConfig{
		StartRecognizeCount:    20,
		StopRecognizeCount:     45,
		PreStartRecognizeCount: 15,
		ActivePercent:          0.8,
		InactivePercent:        0.4,
	})
	defer vad.Release()
	recvCfg := agoraservice.RtcConnectionConfig{
		SubAudio:       true,
		SubVideo:       false,
		ClientRole:     2,
		ChannelProfile: 1,
		//NOTICE: the input audio format of vad is fixed to 16k, 1 channel, 16bit
		SubAudioConfig: &agoraservice.SubscribeAudioConfig{
			SampleRate: 16000,
			Channels:   1,
		},
		ConnectionHandler: &agoraservice.RtcConnectionEventHandler{
			OnConnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
				fmt.Println("recver Connected")
			},
			OnDisconnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
				fmt.Println("recver Disconnected")
			},
			OnUserJoined: func(con *agoraservice.RtcConnection, uid string) {
				fmt.Println("user joined, " + uid)
			},
			OnUserLeft: func(con *agoraservice.RtcConnection, uid string, reason int) {
				fmt.Println("user left, " + uid)
			},
			OnStreamMessage: func(con *agoraservice.RtcConnection, uid string, streamId int, data []byte) {
				fmt.Println("stream message")
			},
			OnStreamMessageError: func(con *agoraservice.RtcConnection, uid string, streamId int, errCode int, missed int, cached int) {
				fmt.Println("stream message error")
			},
		},
		AudioFrameObserver: &agoraservice.RtcConnectionAudioFrameObserver{
			OnPlaybackAudioFrameBeforeMixing: func(con *agoraservice.RtcConnection, channelId string, userId string, frame *agoraservice.PcmAudioFrame) {
				// t.Log("Playback audio frame before mixing")
				out, ret := vad.ProcessPcmFrame(frame)
				if ret < 0 {
					fmt.Println("vad process frame failed")
				}
				if out != nil {
					fmt.Println("vad state %d, out frame time: %d, duration %d\n", ret, out.Timestamp, out.SamplesPerChannel/16)
				} else {
					fmt.Println("vad state %d\n", ret)
				}
			},
		},
		VideoFrameObserver: nil,
	}
	recvCon := agoraservice.NewConnection(&recvCfg)
	defer recvCon.Release()
	recvCon.Connect("", "qitest", "222")

	waitSenderStop.Wait()
	time.Sleep(5 * time.Second)
	senderCon.Disconnect()
	recvCon.Disconnect()

}
