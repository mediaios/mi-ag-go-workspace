package main

import (
	"agora.io/agoraservice"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Stream struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CodecType    string `json:"codec_type"`
	AvgFrameRate string `json:"avg_frame_rate"`
	SampleRate   string `json:"sample_rate"`
	Channels     int    `json:"channels"`
}

type FFProbeOutput struct {
	Streams []Stream `json:"streams"`
}

// 获取MP4文件的媒体信息
func getMediaInfo(inputFile string) (int, int, float64, int, int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", inputFile)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, 0, 0, 0, 0, err
	}

	var ffprobeOutput FFProbeOutput
	if err := json.Unmarshal(out.Bytes(), &ffprobeOutput); err != nil {
		return 0, 0, 0, 0, 0, err
	}

	var width, height, sampleRate, channels int
	var framerate float64

	for _, stream := range ffprobeOutput.Streams {
		if stream.CodecType == "video" {
			width = stream.Width
			height = stream.Height

			framerateParts := strings.Split(stream.AvgFrameRate, "/")
			if len(framerateParts) == 2 {
				numerator, _ := strconv.Atoi(framerateParts[0])
				denominator, _ := strconv.Atoi(framerateParts[1])
				if denominator != 0 {
					framerate = float64(numerator) / float64(denominator)
				}
			}
		} else if stream.CodecType == "audio" {
			sampleRate, _ = strconv.Atoi(stream.SampleRate)
			channels = stream.Channels
		}
	}
	return width, height, framerate, sampleRate, channels, nil
}

// Agora: 计算码率
func RecommendBit(iwidth, iheight, ifps, ichannelType int) float64 {
	term1 := 400 * math.Pow(float64(iwidth*iheight)/(640*360), 0.75)
	term2 := math.Pow(float64(ifps)/15, 0.6)
	term3 := math.Pow(2, float64(ichannelType))
	recommendBit := term1 * term2 * term3
	return recommendBit
}

func main() {

	//mp4Path := flag.String("mp4", "./test_data/henyuandedifang.mp4", "Path to the MP4 file")
	mp4Path := flag.String("mp4", "./test_data/farewell.mp4", "Path to the MP4 file")
	channel := flag.String("channel", "qitest2", "Channel name")
	uid := flag.String("uid", "0", "uid")
	flag.Parse()

	fmt.Printf("MP4 Path: %s, channel: %s, uid: %s \n", *mp4Path, *channel, *uid)

	//inputFile := "./test_data/henyuandedifang.mp4"
	//inputFile := "./test_data/4kvideo.mp4"
	inputFile := *mp4Path
	width, height, framerate, sampleRate, channels, err := getMediaInfo(inputFile)
	if err != nil {
		fmt.Printf("Error getting media info: %v\n", err)
		return
	}

	fmt.Printf("Width: %d, Height: %d, Framerate: %.2f, SampleRate: %d, Channels: %d\n", width, height, framerate, sampleRate, channels)

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
	con.Connect("", *channel, *uid)
	<-conSignal
	sender.Start()

	/*
			samples计算方式：
				*采样率是 44100，因此采集10ms的数据采样个数是 44100/1000*10 = 441 个
		    bufferSize计算方式：
				* sample*bitPerSample/8*channels = 441*16/8*2 = 1764
	*/
	samples := int(float64(sampleRate) / 1000 * 10)
	bufferSize := samples * 16 * channels / 8

	audioFrame := agoraservice.PcmAudioFrame{
		Data:              make([]byte, bufferSize),
		Timestamp:         0,
		SamplesPerChannel: samples,
		BytesPerSample:    16 / 8,
		NumberOfChannels:  channels,
		SampleRate:        sampleRate,
	}
	sender.SetSendBufferSize(1000)

	videoSender := con.GetVideoSender()
	w := width
	h := height
	video_dataSize := w * h * 3 / 2
	video_data := make([]byte, video_dataSize)
	videoBitrate := int(RecommendBit(width, height, int(framerate), 1))
	videoSender.SetVideoEncoderConfig(&agoraservice.VideoEncoderConfig{
		CodecType:         2,
		Width:             w,
		Height:            h,
		Framerate:         int(framerate),
		Bitrate:           videoBitrate,
		MinBitrate:        videoBitrate,
		OrientationMode:   0,
		DegradePreference: 0,
	})
	videoSender.Start()

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
		fmt.Printf("send pcm data, frameDuration: %dms\n", frameDuration.Milliseconds())
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
			mTimestamp := int64(time.Since(startTime).Milliseconds())
			audioFrame.Timestamp = mTimestamp
			sender.SendPcmData(&audioFrame)
			//fmt.Printf("send pcm data, timestamp: %d \n", mTimestamp)

			// 确保帧的发送频率
			//fmt.Printf("send pcm data1, nextFrameTime: %d \n", nextFrameTime.UnixMilli())
			mSleeptime := time.Until(nextFrameTime)
			time.Sleep(mSleeptime)
			//fmt.Printf("send pcm data, mSleeptime: %d \n", mSleeptime.Milliseconds())
			nextFrameTime = nextFrameTime.Add(frameDuration)
			//fmt.Printf("send pcm data, nextFrameTime: %d \n", nextFrameTime.UnixMilli())
		}
	}()

	go func() {
		videoReader := bufio.NewReaderSize(videoOut, video_dataSize)
		frameInterval := time.Second / time.Duration(math.Round(framerate))
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
		fmt.Printf("qizhangDebug,Audio FFmpeg process finished with error: %v\n", err)

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
