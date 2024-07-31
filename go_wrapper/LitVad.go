package main

import (
	"agora.io/agoraservice"
	"fmt"
	"sync"
	"time"
)

func main() {

	bStop := new(bool)
	*bStop = false

	svcCfg := agoraservice.AgoraServiceConfig{
		AppId: "20338919f2ca4af4b1d7ec23d8870b56",
	}
	agoraservice.Init(&svcCfg)

	vad := agoraservice.NewAudioVad(&agoraservice.AudioVadConfig{
		StartRecognizeCount:    10,
		StopRecognizeCount:     6,
		PreStartRecognizeCount: 10,
		ActivePercent:          0.6,
		InactivePercent:        0.2,
	})
	defer vad.Release()

	senderCfg := agoraservice.RtcConnectionConfig{
		SubAudio:       true,
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
		SubAudioConfig: &agoraservice.SubscribeAudioConfig{
			SampleRate: 16000,
			Channels:   1,
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
	//go func() {
	//
	//	frame := agoraservice.PcmAudioFrame{
	//		Data:              make([]byte, 960),
	//		Timestamp:         0,
	//		SamplesPerChannel: 480,
	//		BytesPerSample:    2,
	//		NumberOfChannels:  1,
	//		SampleRate:        48000,
	//	}
	//
	//	defer waitSenderStop.Done()
	//	file, err := os.Open("./test_data/lit_test_pcm.pcm")
	//	if err != nil {
	//		fmt.Println("Error opening file:", err)
	//		return
	//	}
	//	defer file.Close()
	//
	//	sender.AdjustVolume(100)
	//	//sender.SetSendBufferSize(1000)
	//
	//	sendCount := 0
	//	// send 180ms audio data
	//	for i := 0; i < 18; i++ {
	//		dataLen, err := file.Read(frame.Data)
	//		if err != nil || dataLen < 960 {
	//			fmt.Println("Finished reading file:", err)
	//			break
	//		}
	//		sendCount++
	//		sender.SendPcmData(&frame)
	//		//ret := sender.SendPcmData(&frame)
	//		//fmt.Printf("SendPcmData222 %d ret: %d\n", sendCount, ret)
	//	}
	//
	//	// ffmpeg -i lit_test.m4a -ac 1 -ar 48000 -f s16le lit_test_pcm.pcm
	//
	//	firstSendTime := time.Now()
	//	for !(*bStop) {
	//		shouldSendCount := int(time.Since(firstSendTime).Milliseconds()/10) - (sendCount - 18)
	//		fmt.Printf("qidebug, shouldSendCount %d \n", shouldSendCount)
	//
	//		for i := 0; i < shouldSendCount; i++ {
	//			dataLen, err := file.Read(frame.Data)
	//			if err != nil || dataLen < 960 {
	//				fmt.Println("Finished reading file:", err)
	//				file.Seek(0, 0)
	//				i--
	//				continue
	//			}
	//
	//			sendCount++
	//			sender.SendPcmData(&frame)
	//			//ret := sender.SendPcmData(&frame)
	//			//fmt.Printf("SendPcmData %d ret: %d\n", sendCount, ret)
	//		}
	//		fmt.Printf("Sent %d frames this time\n", shouldSendCount)
	//		time.Sleep(50 * time.Millisecond)
	//	}
	//	sender.Stop()
	//}()

	waitSenderStop.Wait()
	time.Sleep(5 * time.Second)
	senderCon.Disconnect()

}
