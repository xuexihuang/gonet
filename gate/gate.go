package gate

import (
	"github.com/xuexihuang/gonet/chanrpc"
	"github.com/xuexihuang/gonet/log"
	"github.com/xuexihuang/gonet/network"
	"net"
	"reflect"
	"time"
)

type Gate struct {
	MaxConnNum      int
	PendingWriteNum int
	MaxMsgLen       uint32
	Processor       network.Processor
	AgentChanRPC    *chanrpc.Server

	// websocket
	WSAddr      string
	HTTPTimeout time.Duration
	CertFile    string
	KeyFile     string

	// tcp
	TCPAddr   string
	LenMsgLen int

	//add by huanglin
	FunNewAgent   func(Agent)
	FunCloseAgent func(Agent)
	FuncMsgRecv   func(interface{}, Agent)
}

func (gate *Gate) SetFun(Fun1 func(Agent), Fun2 func(Agent), Fun3 func(interface{}, Agent)) {
	gate.FunNewAgent = Fun1
	gate.FunCloseAgent = Fun2
	gate.FuncMsgRecv = Fun3
}

func (gate *Gate) Run(closeSig chan bool) {
	var wsServer *network.WSServer
	if gate.WSAddr != "" {
		wsServer = new(network.WSServer)
		wsServer.Addr = gate.WSAddr
		wsServer.MaxConnNum = gate.MaxConnNum
		wsServer.PendingWriteNum = gate.PendingWriteNum
		wsServer.MaxMsgLen = gate.MaxMsgLen
		wsServer.HTTPTimeout = gate.HTTPTimeout
		wsServer.CertFile = gate.CertFile
		wsServer.KeyFile = gate.KeyFile
		wsServer.NewAgent = func(conn *network.WSConn) network.Agent {
			a := &agent{conn: conn, gate: gate}
			/*if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}*/
			gate.FunNewAgent(a)
			return a
		}
	}

	var tcpServer *network.TCPServer
	if gate.TCPAddr != "" {
		tcpServer = new(network.TCPServer)
		tcpServer.Addr = gate.TCPAddr
		tcpServer.MaxConnNum = gate.MaxConnNum
		tcpServer.PendingWriteNum = gate.PendingWriteNum
		tcpServer.LenMsgLen = gate.LenMsgLen
		tcpServer.MaxMsgLen = gate.MaxMsgLen
		tcpServer.UsePacketMode = gate.Processor.UsePacketMode()
		tcpServer.NewAgent = func(conn *network.TCPConn) network.Agent {
			a := &agent{conn: conn, gate: gate}
			/*if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}*/
			gate.FunNewAgent(a)
			return a
		}
	}

	if wsServer != nil {
		wsServer.Start()
	}
	if tcpServer != nil {
		tcpServer.Start()
	}
	<-closeSig
	if wsServer != nil {
		wsServer.Close()
	}
	if tcpServer != nil {
		tcpServer.Close()
	}
}

func (gate *Gate) OnDestroy() {}

type agent struct {
	conn     network.Conn
	gate     *Gate
	userData interface{}
}

func (a *agent) Run() {
	for {
		data, err := a.conn.ReadMsg()
		if err != nil {
			log.Debug("read message: %v", err)
			break
		}
		log.Debug("read message success")
		if a.gate.Processor != nil {
			msg, err := a.gate.Processor.Unmarshal(data)
			if err != nil {
				log.Debug("unmarshal message error: %v", err)
				break
			}
			a.gate.FuncMsgRecv(msg, a)
			/*err = a.gate.Processor.Route(msg, a)
			if err != nil {
				log.Debug("route message error: %v", err)
				break
			}*/
		}
	}
}

func (a *agent) OnClose() {

	/*if a.gate.AgentChanRPC != nil {
		err := a.gate.AgentChanRPC.Call0("CloseAgent", a)
		if err != nil {
			log.Error("chanrpc error: %v", err)
		}
	}*/
	a.gate.FunCloseAgent(a)
}

func (a *agent) WriteMsg(msg interface{}) {
	if a.gate.Processor != nil {
		data, err := a.gate.Processor.Marshal(msg)
		if err != nil {
			log.Error("marshal message %v error: %v", reflect.TypeOf(msg), err)
			return
		}
		err = a.conn.WriteMsg(data...)
		if err != nil {
			log.Error("write message %v error: %v", reflect.TypeOf(msg), err)
		}
	}
}

func (a *agent) LocalAddr() net.Addr {
	return a.conn.LocalAddr()
}

func (a *agent) RemoteAddr() net.Addr {
	return a.conn.RemoteAddr()
}

func (a *agent) Close() {
	a.conn.Close()
}

func (a *agent) Destroy() {
	a.conn.Destroy()
}

func (a *agent) UserData() interface{} {
	return a.userData
}

func (a *agent) SetUserData(data interface{}) {
	a.userData = data
}
