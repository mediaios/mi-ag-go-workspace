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

	// video sender
	videoSender := con.GetVideoSender()
	w := 640
	h := 360
	video_dataSize := w * h * 3 / 2
	video_data := make([]byte,dataSize)
	videoSender.SetVideoEncoderConfig(&VideoEncoderConfig{
		CodecType:         2,
		Width:             640,
		Height:            360,
		Framerate:         30,
		Bitrate:           1212,
		MinBitrate:        800,
		OrientationMode:   0,
		DegradePreference: 0,
	})
	videoSender.Start()


	inputFile := "./test_data/sony_640.mp4"

	// 启动FFmpeg进程以分离视频数据
	videoCmd := exec.Command("ffmpeg", "-i", inputFile, "-f", "rawvideo", "-pix_fmt", "yuv420p", "-")
	videoOut, err := videoCmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating video output pipe: %v\n", err)
		return
	}
	videoErr, err := videoCmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating video error pipe: %v\n", err)
		return
	}

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
	if err := videoCmd.Start(); err != nil {
		fmt.Printf("Error starting video FFmpeg process: %v\n", err)
		return
	}
	if err := audioCmd.Start(); err != nil {
		fmt.Printf("Error starting audio FFmpeg process: %v\n", err)
		return
	}


	// 处理音频数据
	go func() {
		defer func() {
			sender.Stop()
			con.Disconnect()
			agoraservice.Destroy()
		}()

		audioReader := bufio.NewReaderSize(audioOut, 8192)
		startTime := time.Now()
		frameDuration := time.Duration(audioFrame.SamplesPerChannel) * time.Second / time.Duration(audioFrame.SampleRate)

		for {
			_, err := io.ReadFull(audioReader, audioFrame.Data)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Audio data read complete")
				} else {
					fmt.Printf("Error reading audio data: %v\n", err)
				}
				break
			}

			// 增加时间戳防止帧的累积
			audioFrame.Timestamp = int64(time.Since(startTime) / time.Millisecond)
			sender.SendPcmData(&audioFrame)
			time.Sleep(frameDuration - time.Since(startTime)%frameDuration) // 精确控制发送频率
		}
	}()

	go func() {
		videoReader := bufio.NewReaderSize(videoOut, 345600) // 适应视频帧大小的缓冲区
		frameInterval := time.Second / 30 // 假设视频帧率为30fps
		lastFrameTime := time.Now()

		for {
			_, err := io.ReadFull(videoReader, video_data)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Video data read complete")
				} else {
					fmt.Printf("Error reading video data: %v\n", err)
				}
				break
			}

			time.Sleep(frameInterval - time.Since(lastFrameTime))
			lastFrameTime = time.Now()

			videoSender.SendVideoFrame(&VideoFrame{
				Buffer:    video_dataSize,
				Width:     w,
				Height:    h,
				YStride:   w,
				UStride:   w / 2,
				VStride:   w / 2,
				Timestamp: 0,
			})
			time.Sleep(33 * time.Millisecond)

		}

	}()

	// 读取并打印 FFmpeg 的错误输出
	go func() {
		audioErrReader := bufio.NewReader(audioErr)
		printFFmpegErrorOutput(audioErrReader)
	}()

	go func() {
		videoErrReader := bufio.NewReader(videoErr)
		printFFmpegErrorOutput(videoErrReader)
	}()

	// 等待 FFmpeg 进程完成
	if err := videoCmd.Wait(); err != nil {
		fmt.Printf("Video FFmpeg process finished with error: %v\n", err)
	}
	if err := audioCmd.Wait(); err != nil {
		fmt.Printf("Audio FFmpeg process finished with error: %v\n", err)

		sender.Stop()
		videoSender.Stop()
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
