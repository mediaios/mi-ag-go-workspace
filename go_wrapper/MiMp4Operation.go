package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"time"

	"bufio"
	"agora.io/agoraservice"
)

func main() {
	svcCfg := agoraservice.AgoraServiceConfig{
		AppId: "20338919f2ca4af4b1d7ec23d8870b56",
	}
	agoraservice.Init(&svcCfg)
	conCfg := agoraservice.RtcConnectionConfig{
		SubAudio:      false,
		SubVideo:      false,
		ClientRole:    1,
		ChannelProfile: 1,
	}
	conSignal := make(chan struct{})
	connHandler := agoraservice.RtcConnectionEventHandler{
		OnConnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Println("Connected")
			conSignal <- struct{}{}
		},
		OnDisconnected: func(con *agoraservice.RtcConnection, info *agoraservice.RtcConnectionInfo, reason int) {
			fmt.Println("Disconnected")
		},
		OnUserJoined: func(con *agoraservice.RtcConnection, uid string) {
			fmt.Println("user joined, " + uid)
		},
		OnUserLeft: func(con *agoraservice.RtcConnection, uid string, reason int) {
			fmt.Println("user left, " + uid)
		},
	}
	conCfg.ConnectionHandler = &connHandler
	con := agoraservice.NewConnection(&conCfg)
	defer con.Release()
	sender := con.NewPcmSender()
	defer sender.Release()
	con.Connect("", "qitest", "0")
	<-conSignal
	sender.Start()

	audioFrame := agoraservice.PcmAudioFrame{
		Data:              make([]byte, 1920),
		Timestamp:         0,
		SamplesPerChannel: 480,
		BytesPerSample:    2,
		NumberOfChannels:  2,
		SampleRate:        48000,
	}
	sender.SetSendBufferSize(1000)

	inputFile := "./test_data/sony_640.mp4"

	// 启动FFmpeg进程以分离音频数据
	audioCmd := exec.Command("ffmpeg", "-i", inputFile, "-f", "s16le", "-acodec", "pcm_s16le", "-")
	audioOut, err := audioCmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating audio output pipe: %v\n", err)
		return
	}
	audioErr, err := audioCmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating audio error pipe: %v\n", err)
		return
	}

	// 启动FFmpeg进程
	if err := audioCmd.Start(); err != nil {
		fmt.Printf("Error starting audio FFmpeg process: %v\n", err)
		return
	}

	// 处理音频数据
	go func() {
		audioReader := bufio.NewReader(audioOut)
		defer func() {
			sender.Stop()
			con.Disconnect()
			agoraservice.Destroy()
		}()

		for {
			// 读取音频数据
			dataLen, err := audioReader.Read(audioFrame.Data)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Audio data read complete")
				} else {
					fmt.Printf("Error reading audio data: %v\n", err)
				}
				break
			}

			// 检查读取的数据长度是否符合预期
			if dataLen < len(audioFrame.Data) {
				fmt.Println("Incomplete audio frame")
				break
			}

			// 发送音频数据给Agora SDK
			sender.SendPcmData(&audioFrame)

			// 控制音频发送频率，每隔10毫秒发送一次
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// 打印 FFmpeg 的错误输出
	go func() {
		audioErrReader := bufio.NewReader(audioErr)
		printFFmpegErrorOutput(audioErrReader)
	}()

	// 等待 FFmpeg 进程完成
	if err := audioCmd.Wait(); err != nil {
		fmt.Printf("Audio FFmpeg process finished with error: %v\n", err)
	}
}

// 打印 FFmpeg 的错误输出
func printFFmpegErrorOutput(reader *bufio.Reader) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	fmt.Println(buf.String())
}
