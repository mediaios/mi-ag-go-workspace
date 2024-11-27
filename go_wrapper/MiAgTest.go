package main

import (
	"fmt"
	agoraservice "github.com/AgoraIO-Extensions/Agora-Golang-Server-SDK/v2/go_sdk/agoraservice"
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

	svcCfg := agoraservice.NewAgoraServiceConfig()
	svcCfg.AppId = "20338919f2ca4af4b1d7ec23d8870b56"
	svcCfg.LogPath = "./agora_rtc_log/agorasdk.log"
	svcCfg.LogSize = 512 * 1024
	agoraservice.Initialize(svcCfg)
	defer agoraservice.Release()

	mediaNodeFactory := agoraservice.NewMediaNodeFactory()
	defer mediaNodeFactory.Release()

	conCfg := agoraservice.RtcConnectionConfig{
		AutoSubscribeAudio: true,
		AutoSubscribeVideo: false,
		ClientRole:         agoraservice.ClientRoleBroadcaster,
		ChannelProfile:     agoraservice.ChannelProfileLiveBroadcasting,
	}
	conSignal := make(chan struct{})
	conHandler := &agoraservice.RtcConnectionObserver{
		OnConnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
			// do something
			fmt.Println("Connected")
			conSignal <- struct{}{}
		},
		OnDisconnected: func(con *agoraservice.RtcConnection, conInfo *agoraservice.RtcConnectionInfo, reason int) {
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

	audioObserver := &agoraservice.AudioFrameObserver{
		OnPlaybackAudioFrameBeforeMixing: func(localUser *agoraservice.LocalUser, channelId string, userId string, frame *agoraservice.AudioFrame) bool {
			// do something
			fmt.Printf("Playback audio frame before mixing, from userId %s\n", userId)
			return true
		},
	}

	uerObserver := &agoraservice.LocalUserObserver{
		OnStreamMessage: func(localUser *agoraservice.LocalUser, uid string, streamId int, data []byte) {

		},
	}

	con := agoraservice.NewRtcConnection(&conCfg)
	defer con.Release()

	localUser := con.GetLocalUser()
	localUser.SetPlaybackAudioFrameBeforeMixingParameters(1, 16000)
	con.RegisterObserver(conHandler)
	localUser.RegisterAudioFrameObserver(audioObserver)
	localUser.RegisterLocalUserObserver(uerObserver)

	sender := mediaNodeFactory.NewAudioPcmDataSender()
	defer sender.Release()
	track := agoraservice.NewCustomAudioTrackPcm(sender)
	defer track.Release()

	localUser.SetAudioScenario(agoraservice.AudioScenarioChorus)
	con.Connect("", "qitest", "0")
	<-conSignal

	track.SetEnabled(true)
	localUser.PublishAudio(track)

	frame := agoraservice.AudioFrame{
		Type:              agoraservice.AudioFrameTypePCM16,
		SamplesPerChannel: 160,
		BytesPerSample:    2,
		Channels:          1,
		SamplesPerSec:     16000,
		Buffer:            make([]byte, 320),
		RenderTimeMs:      0,
	}
	file, err := os.Open("./test_data/demo.pcm")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	track.AdjustPublishVolume(100)

	sendCount := 0
	// send 180ms audio data
	for i := 0; i < 18; i++ {
		dataLen, err := file.Read(frame.Buffer)
		if err != nil || dataLen < 320 {
			fmt.Println("Finished reading file:", err)
			break
		}
		sendCount++
		ret := sender.SendAudioPcmData(&frame)
		fmt.Printf("SendPcmData222 %d ret: %d\n", sendCount, ret)
	}

	// ffmpeg -i lit_test.m4a -ac 1 -ar 48000 -f s16le lit_test_pcm.pcm

	firstSendTime := time.Now()
	for !(*bStop) {
		shouldSendCount := int(time.Since(firstSendTime).Milliseconds()/10) - (sendCount - 18)
		fmt.Printf("qidebug, shouldSendCount %d \n", shouldSendCount)

		for i := 0; i < shouldSendCount; i++ {
			dataLen, err := file.Read(frame.Buffer)
			if err != nil || dataLen < 320 {
				fmt.Println("Finished reading file:", err)
				file.Seek(0, 0)
				i--
				continue
			}

			sendCount++
			ret := sender.SendAudioPcmData(&frame)
			fmt.Printf("SendPcmData %d ret: %d\n", sendCount, ret)
		}
		fmt.Printf("Sent %d frames this time\n", shouldSendCount)
		time.Sleep(50 * time.Millisecond)
	}
	localUser.UnpublishAudio(track)
	track.SetEnabled(false)
	con.Disconnect()
}
