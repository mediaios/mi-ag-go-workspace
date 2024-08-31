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
		AppId:         "20338919f2ca4af4b1d7ec23d8870b56",
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
			fmt.Println("QiDebug, Connected")
			conSignal <- struct{}{}
		},
		OnDisconnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
			// do something
			fmt.Println("Disconnected")
			conSignal <- struct{}{}
		},
		OnUserJoined: func(con *agoraservice.RtcConnection, uid string) {
			fmt.Println("user joined, " + uid)
			conSignal <- struct{}{}
		},
		OnUserLeft: func(con *agoraservice.RtcConnection, uid string, reason int) {
			fmt.Println("user left, " + uid)
			conSignal <- struct{}{}
		},
		OnStreamMessage: func(con *agoraservice.RtcConnection, uid string, streamId int, data []byte) {

			fmt.Printf("receive dataStream message %d: %s\n", streamId, string(data))
			conSignal <- struct{}{}
		},
		OnStreamMessageError: func(con *agoraservice.RtcConnection, uid string, streamId int, errCode int, missed int, cached int) {
			fmt.Printf("QiDebug, occurStreamMessageError, uid:%d, streamId:%d, error:%d, missed:%d, cached:%d\n", uid, streamId, errCode, missed, cached)
			conSignal <- struct{}{}
		},
	}
	conCfg.ConnectionHandler = &conHandler
	con := agoraservice.NewConnection(&conCfg)
	defer con.Release()

	var stopSend *bool = new(bool)
	*stopSend = false
	con.Connect("", "qitest", "222")

	streamId, ret := con.CreateDataStream(true, true)
	if ret != 0 {
		fmt.Printf("create stream failed")
		return
	}

	for !*stopSend {
		// 获取当前时间戳并格式化为字符串
		currentTime := time.Now().UnixNano() / int64(time.Millisecond)
		str := fmt.Sprintf("qi-dataStream-sei-%f", float64(currentTime)/1000.0)

		// 将字符串转换为字节数组
		data := []byte(str)

		// 发送数据流消息
		con.SendStreamMessage(streamId, data)

		time.Sleep(250 * time.Millisecond)
	}

	<-conSignal

	con.Disconnect()

	agoraservice.Destroy()
}
