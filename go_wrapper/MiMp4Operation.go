package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

func main() {
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

	// 处理视频数据
	go func() {
		frameSize := 640 * 360 * 3 / 2 // 假设 YUV420P 格式和 1080p 分辨率
		videoReader := bufio.NewReader(videoOut)
		for {
			buf := make([]byte, frameSize)
			_, err := io.ReadFull(videoReader, buf)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Video data read complete")
				} else {
					fmt.Printf("Error reading video data: %v\n", err)
				}
				break
			}
			// 处理一帧视频数据
			handleVideoFrame(buf)
		}
	}()

	// 处理音频数据
	go func() {
		buf := make([]byte, 4096) // 根据需要调整缓冲区大小
		audioReader := bufio.NewReader(audioOut)
		for {
			n, err := audioReader.Read(buf)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Audio data read complete")
				} else {
					fmt.Printf("Error reading audio data: %v\n", err)
				}
				break
			}
			// 处理一段音频数据
			handleAudioFrame(buf[:n])
		}
	}()

	// 读取并打印 FFmpeg 的错误输出
	go func() {
		videoErrReader := bufio.NewReader(videoErr)
		audioErrReader := bufio.NewReader(audioErr)
		printFFmpegErrorOutput(videoErrReader)
		printFFmpegErrorOutput(audioErrReader)
	}()

	// 等待 FFmpeg 进程完成
	if err := videoCmd.Wait(); err != nil {
		fmt.Printf("Video FFmpeg process finished with error: %v\n", err)
	}
	if err := audioCmd.Wait(); err != nil {
		fmt.Printf("Audio FFmpeg process finished with error: %v\n", err)
	}
}

// 处理视频帧的回调函数
func handleVideoFrame(frame []byte) {
	// 在这里处理每一帧视频数据
	fmt.Println("Received a video frame")
}

// 处理音频帧的回调函数
func handleAudioFrame(frame []byte) {
	// 在这里处理每一帧音频数据
	fmt.Println("Received an audio frame")
}

// 打印 FFmpeg 的错误输出
func printFFmpegErrorOutput(reader *bufio.Reader) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	fmt.Println(buf.String())
}