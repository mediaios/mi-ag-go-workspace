package main

import (
	"agora.io/agoraservice"
	"fmt"
	"os"
	"sync"
	"time"
)

func main() {

	svcCfg := agoraservice.AgoraServiceConfig{
		AppId: "20338919f2ca4af4b1d7ec23d8870b56",
	}
	agoraservice.Init(&svcCfg)
	senderCfg := agoraservice.RtcConnectionConfig{
		SubAudio:       false,
		SubVideo:       false,
		ClientRole:     1,
		ChannelProfile: 1,
		ConnectionHandler: &agoraservice.RtcConnectionEventHandler{
			OnConnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
				fmt.Println("Connected")
			},
			OnDisconnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
				fmt.Println("Disconnected")
			},
		},
	}

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
		defer waitSenderStop.Done()
		file, err := os.Open("./test_data/vad_test.pcm")
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		data := make([]byte, 320)
		for !*stopSend {
			dataLen, err := file.Read(data)
			if err != nil || dataLen < 320 {
				break
			}
			sender.SendPcmData(&agoraservice.PcmAudioFrame{
				Data:              data,
				Timestamp:         0,
				SamplesPerChannel: 160,
				BytesPerSample:    2,
				NumberOfChannels:  1,
				SampleRate:        16000,
			})
			time.Sleep(10 * time.Millisecond)
		}
		sender.Stop()
	}()

	vad := agoraservice.NewAudioVad(&agoraservice.AudioVadConfig{
		StartRecognizeCount:    10,
		StopRecognizeCount:     6,
		PreStartRecognizeCount: 10,
		ActivePercent:          0.6,
		InactivePercent:        0.2,
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
