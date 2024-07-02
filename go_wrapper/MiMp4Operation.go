package main

import (
	"agora.io/agoraservice"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"time"
)

func main() {
	svcCfg := agoraservice.AgoraServiceConfig{
		AppId: "20338919f2ca4af4b1d7ec23d8870b56",
	}
	agoraservice.Init(&svcCfg)
	conCfg := agoraservice.RtcConnectionConfig{
		SubAudio:       false,
		SubVideo:       false,
		ClientRole:     1,
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
		Data:              make([]byte, 1764),
		Timestamp:         0,
		SamplesPerChannel: 441,
		BytesPerSample:    2,
		NumberOfChannels:  2,
		SampleRate:        44100,
	}
	sender.SetSendBufferSize(1000)

	videoSender := con.GetVideoSender()
	w := 1280
	h := 720
	video_dataSize := w * h * 3 / 2
	video_data := make([]byte, video_dataSize)
	videoSender.SetVideoEncoderConfig(&agoraservice.VideoEncoderConfig{
		CodecType:         2,
		Width:             w,
		Height:            h,
		Framerate:         24,
		Bitrate:           3000,
		MinBitrate:        2800,
		OrientationMode:   0,
		DegradePreference: 0,
	})
	videoSender.Start()

	inputFile := "./test_data/henyuandedifang.mp4"

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

	if err := videoCmd.Start(); err != nil {
		fmt.Printf("Error starting video FFmpeg process: %v\n", err)
		return
	}
	if err := audioCmd.Start(); err != nil {
		fmt.Printf("Error starting audio FFmpeg process: %v\n", err)
		return
	}

	startTime := time.Now()

	go func() {
		defer func() {
			sender.Stop()
			con.Disconnect()
			agoraservice.Destroy()
		}()

		audioReader := bufio.NewReaderSize(audioOut, 8192)
		frameDuration := time.Duration(audioFrame.SamplesPerChannel) * time.Second / time.Duration(audioFrame.SampleRate)
		nextFrameTime := startTime

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

			// 计算音频帧的时间戳
			audioFrame.Timestamp = int64(time.Since(startTime).Milliseconds())
			sender.SendPcmData(&audioFrame)

			// 确保帧的发送频率
			time.Sleep(time.Until(nextFrameTime))
			nextFrameTime = nextFrameTime.Add(frameDuration)
		}
	}()

	go func() {
		videoReader := bufio.NewReaderSize(videoOut, video_dataSize)
		frameInterval := time.Second / 24
		nextFrameTime := startTime

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

			videoFrame := agoraservice.VideoFrame{
				Buffer:    video_data,
				Width:     w,
				Height:    h,
				YStride:   w,
				UStride:   w / 2,
				VStride:   w / 2,
				Timestamp: int64(time.Since(startTime).Milliseconds()),
			}

			videoSender.SendVideoFrame(&videoFrame)

			// 确保帧的发送频率
			time.Sleep(time.Until(nextFrameTime))
			nextFrameTime = nextFrameTime.Add(frameInterval)
		}
	}()

	go func() {
		audioErrReader := bufio.NewReader(audioErr)
		printFFmpegErrorOutput(audioErrReader)
	}()

	go func() {
		videoErrReader := bufio.NewReader(videoErr)
		printFFmpegErrorOutput(videoErrReader)
	}()

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

func printFFmpegErrorOutput(reader *bufio.Reader) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	fmt.Println(buf.String())
}
