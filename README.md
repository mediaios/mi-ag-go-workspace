# mi-ag-go-workspace
poc for go sdk 

## 如何运行

### linux

```
export LD_LIBRARY_PATH=/home/golang/src/mi-ag-go-workspace/agora_sdk

cd go_wrapper
go build MiAgTest.go
./MiAgTest
```

### mac

```
# 在 go_wrapper 目录下执行命令 
export CGO_LDFLAGS_ALLOW="-Wl,-rpath,.*"
export CGO_LDFLAGS="-Wl,-rpath,../agora_sdk_mac"
```




##FAQ

Q1: 运行demo的过程中遇到如下错误该如何解决？

![](https://i.imgur.com/2mYrdYz.png)

A1：需要安装
![](https://i.imgur.com/2vSFD9D.png)