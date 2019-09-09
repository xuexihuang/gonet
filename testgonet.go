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