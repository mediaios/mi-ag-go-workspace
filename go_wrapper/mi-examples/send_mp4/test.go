package main

// #cgo CFLAGS: -I/usr/local/Cellar/ffmpeg/7.0.2/include
// #cgo LDFLAGS: -L/usr/local/Cellar/ffmpeg/7.0.2/lib -lavcodec -lavformat -lavutil -lswresample
//#include <libavformat/avformat.h>
import "C"
import "fmt"

func main() {
	fmt.Println(C.av_version_info())
}
