package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"agora.io/agoraservice"
	"time"
)

func main() {
	svcCfg := agoraservice.AgoraServiceConfig{
		AppId:"20338919f2ca4af4b1d7ec23d8870b56",
	}
	agoraservice.Init(&svcCfg)
	conCfg := agoraservice.RtcConnectionConfig{
		SubAudio:		false,
		SubVideo:		false,
		ClientRole:		1,
		ChannelProfile:	1,
	}
	conSignal := make(chan struct{})
	connHandler := agoraservice.RtcConnectionEventHandler{
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

	//// 启动FFmpeg进程以分离视频数据
	//videoCmd := exec.Command("ffmpeg", "-i", inputFile, "-f", "rawvideo", "-pix_fmt", "yuv420p", "-")
	//videoOut, err := videoCmd.StdoutPipe()
	//if err != nil {
	//	fmt.Printf("Error creating video output pipe: %v\n", err)
	//	return
	//}
	//videoErr, err := videoCmd.StderrPipe()
	//if err != nil {
	//	fmt.Printf("Error creating video error pipe: %v\n", err)
	//	return
	//}

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

	//// 启动FFmpeg进程
	//if err := videoCmd.Start(); err != nil {
	//	fmt.Printf("Error starting video FFmpeg process: %v\n", err)
	//	return
	//}
	if err := audioCmd.Start(); err != nil {
		fmt.Printf("Error starting audio FFmpeg process: %v\n", err)
		return
	}

	//// 处理视频数据
	//go func() {
	//	frameSize := 640 * 360 * 3 / 2 // 假设 YUV420P 格式和 1080p 分辨率
	//	videoReader := bufio.NewReader(videoOut)
	//	for {
	//		buf := make([]byte, frameSize)
	//		_, err := io.ReadFull(videoReader, buf)
	//		if err != nil {
	//			if err == io.EOF {
	//				fmt.Println("Video data read complete")
	//			} else {
	//				fmt.Printf("Error reading video data: %v\n", err)
	//			}
	//			break
	//		}
	//		// 处理一帧视频数据
	//		handleVideoFrame(buf)
	//	}
	//}()

	// 处理音频数据
	go func() {
		defer func() {
			sender.Stop()
			con.Disconnect()
			agoraservice.Destroy()
		}()

		audioReader := bufio.NewReaderSize(audioOut, 16384) // 增加缓冲区大小
		for {
			dataLen, err := io.ReadFull(audioReader, audioFrame.Data)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Audio data read complete")
				} else {
					fmt.Printf("Error reading audio data: %v\n", err)
				}
				break
			}

			// 检查读取的数据长度是否符合预期
			fmt.Printf("dataLen: %d\n", dataLen)
			if dataLen < len(audioFrame.Data) {
				fmt.Println("Incomplete audio frame, filling with silence")
				for i := dataLen; i < len(audioFrame.Data); i++ {
					audioFrame.Data[i] = 0 // 填充静音
				}
			}

			sender.SendPcmData(&audioFrame)
			time.Sleep(8 * time.Millisecond) // 控制发送频率，每隔10毫秒发送一次
		}
	}()

	// 读取并打印 FFmpeg 的错误输出
	go func() {
		//videoErrReader := bufio.NewReader(videoErr)
		audioErrReader := bufio.NewReader(audioErr)
		//printFFmpegErrorOutput(videoErrReader)
		printFFmpegErrorOutput(audioErrReader)
	}()

	//// 等待 FFmpeg 进程完成
	//if err := videoCmd.Wait(); err != nil {
	//	fmt.Printf("Video FFmpeg process finished with error: %v\n", err)
	//}
	if err := audioCmd.Wait(); err != nil {
		fmt.Printf("Audio FFmpeg process finished with error: %v\n", err)

		sender.Stop()
		con.Disconnect()

		agoraservice.Destroy()
	}
}

//// 处理视频帧的回调函数
//func handleVideoFrame(frame []byte) {
//	// 在这里处理每一帧视频数据
//	//fmt.Println("Received a video frame")
//}


// 打印 FFmpeg 的错误输出
func printFFmpegErrorOutput(reader *bufio.Reader) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	fmt.Println(buf.String())
}
