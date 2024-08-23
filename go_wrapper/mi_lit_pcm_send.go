package main

import (
	"agora.io/agoraservice"
	"fmt"
	"os"
	"os/signal"
	"time"
)

func main() {
	bStop := new(bool)
	*bStop = false

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		*bStop = true
		fmt.Println("Application terminated")
		os.Exit(0)
	}()

	svcCfg := agoraservice.AgoraServiceConfig{
		AppId:         "",
		AudioScenario: agoraservice.AUDIO_SCENARIO_CHORUS,
		LogPath:       "./agora_rtc_log/agorasdk.log",
		LogSize:       512 * 1024,
	}
	agoraservice.Init(&svcCfg)

	conCfg := agoraservice.RtcConnectionConfig{
		SubAudio:       true,
		SubVideo:       false,
		ClientRole:     1,
		ChannelProfile: 1,

		SubAudioConfig: &agoraservice.SubscribeAudioConfig{
			SampleRate: 16000,
			Channels:   1,
		},
	}

	conSignal := make(chan struct{})
	conHandler := agoraservice.RtcConnectionEventHandler{
		OnConnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
			// do something
			fmt.Println("Connected")
			conSignal <- struct{}{}
		},
		OnDisconnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
			// do something
			fmt.Println("Disconnected")
		},
		OnUserJoined: func(con *agoraservice.RtcConnection, uid string) {
			fmt.Println("user joined, " + uid)
		},
		OnUserLeft: func(con *agoraservice.RtcConnection, uid string, reason int) {
			fmt.Println("user left, " + uid)
		},
	}
	conCfg.ConnectionHandler = &conHandler
	conCfg.AudioFrameObserver = &agoraservice.RtcConnectionAudioFrameObserver{
		OnPlaybackAudioFrameBeforeMixing: func(con *agoraservice.RtcConnection, channelId string, userId string, frame *agoraservice.PcmAudioFrame) {
			// do something
			fmt.Printf("Playback audio frame before mixing, from userId %s\n", userId)
		},
	}
	con := agoraservice.NewConnection(&conCfg)
	defer con.Release()

	sender := con.NewPcmSender()
	defer sender.Release()

	con.Connect("", "qitest", "0")
	<-conSignal
	sender.Start()

	frame := agoraservice.PcmAudioFrame{
		Data:              make([]byte, 480),
		Timestamp:         0,
		SamplesPerChannel: 240,
		BytesPerSample:    2,
		NumberOfChannels:  1,
		SampleRate:        24000,
	}

	file, err := os.Open("./test_data/full.pcm")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	sender.AdjustVolume(100)

	sendCount := 0
	firstSendTime := time.Now()

	for !(*bStop) {
		// Calculate the number of frames that should be sent based on the elapsed time
		shouldSendCount := int(time.Since(firstSendTime).Milliseconds()/10) - sendCount
		fmt.Printf("qidebug, shouldSendCount %d \n", shouldSendCount)

		for i := 0; i < shouldSendCount; i++ {
			dataLen, err := file.Read(frame.Data)
			if err != nil || dataLen < 480 {
				if err != nil {
					fmt.Println("Error reading file:", err)
				} else {
					fmt.Println("Finished reading file")
				}
				*bStop = true // Set the stop flag to true to end the loop
				break
			}

			sendCount++
			ret := sender.SendPcmData(&frame)
			fmt.Printf("SendPcmData %d ret: %d\n", sendCount, ret)
		}

		if *bStop {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	sender.Stop()
	con.Disconnect()
	agoraservice.Destroy()
}
