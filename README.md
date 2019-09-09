# gonet
一个go写的网络库，支持protobuf和json协议格式注册
# 先来看看例子
```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"github.com/xuexihuang/gonet/gate"
	"github.com/xuexihuang/gonet/network/json"
)

var Processor = json.NewProcessor()

func init() {
	Processor.Register(&Login{})
}

//client to server
type Login struct {
	UserName string
	PassWord string
}

type GateNet struct {
	*gate.Gate
	CloseSig chan bool
	Wg       sync.WaitGroup
}

func Initsever() *GateNet {
	gatenet := new(GateNet)
	gatenet.Gate = &gate.Gate{
		MaxConnNum:      10000,//最大连接数量
		PendingWriteNum: 2000,
		MaxMsgLen:       4096,//最大消息长度
		WSAddr:          "",//ws ip地址，和tcp二选一
		HTTPTimeout:     0,
		CertFile:        "",
		KeyFile:         "",
		TCPAddr:         "127.0.0.1:3565",
		LenMsgLen:       2,//消息长度字节数
		Processor:       Processor,//协议解析器对象，demo中使用的是json解析器，也可以是protobuf解析器
	}
	gatenet.CloseSig = make(chan bool, 1)
	return gatenet
}

func (gt *GateNet) SetMsgFun(Fun1 func(gate.Agent), Fun2 func(gate.Agent), Fun3 func(interface{}, gate.Agent)) {
	gt.Gate.SetFun(Fun1, Fun2, Fun3)
}
func (gt *GateNet) Runloop() {
	gt.Wg.Add(1)
	gt.Run(gt.CloseSig)
	gt.Wg.Done()
}
func (gt *GateNet) CloseGate() {
	gt.CloseSig <- true
	gt.Wg.Wait()
	gt.Gate.OnDestroy()
}

func NewAgent(a gate.Agent) {

	fmt.Println("one linkder")
}
func CloseAgent(a gate.Agent) {
	fmt.Println("one dislinkder")
}
func DataRecv(data interface{}, a gate.Agent) {
	msgType := reflect.TypeOf(data)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		fmt.Println("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	fmt.Println("one pack is", msgID)
}
func main() {
	gatenet := Initsever()//注册一个gonet实例
	gatenet.SetMsgFun(NewAgent, CloseAgent, DataRecv)//设置实例的回调函数，分别是连接到底，连接关闭，数据到底
	go gatenet.Runloop()//开启实例循环
	// 等待close信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	sig := <-c
	fmt.Println("Leaf closing down ", sig)
	gatenet.CloseGate()//释放实例
}
```
上面代码的Initsever 就是设置一些必要参数，服务器其中的必要参数，比如ip地址，端口号，最大连接数量。
SetMsgFun 就是设置实例的回调函数，分别是连接到底，连接关闭，数据到底。注意连接关闭是通知事件，内部早已经把连接资源释放了，这是释放后的通知事件，连接到达事件也是通知事件

# gonet为什么采用回调函数方式
本人多年使用c的libuv和libevent网络库的经验，优秀的网络库接口大抵都是采用回调接口，优势是回调函数接口和epoll的异步模式完美结合组成反应堆模型。在golang中，回调函数只需要处理连接，关闭，数据三个
事件就可以实现和业务的无缝连接。如果采用的是actor模式的业务模式，在回调函数里面直接把三类事件通过邮箱告诉相应的actor就可以。
# 为什么协议解析部分也放到网络库中实现
传统的c语言网络库，比如libuv和libevent是不会在网络库中解析协议数据的。可是为什么这里的网络库把这部分功能加上呢，原因是golang处理协议部分特别简单，
统的c语言cpoll在接收数据的数量上没办法控制数量，只能先暂缓存储起来，然后check一下够不够一个协议的字数，如果够，就移位存储。 
可是在golang可以直接调用readatleast或者readfull。指定读取字节数。非常方便。主要是因为golang阻塞式的读写封装在底层做了很多方便应用层的封装

# 回调函数的Agent怎么使用
这是一个客户连接的接口，接口有发送数据，关闭连接的函数，只要调用相应的函数就能实现相应功能。发送数据和关闭连接都是异步的，内部是通过channel通道发送数据和命令到底层网络模块。