package packet

import (
	"github.com/xuexihuang/gonet/chanrpc"
	"github.com/xuexihuang/gonet/log"
	"github.com/xuexihuang/gonet/network"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
)

type Packet interface {
	Protocol() uint8
	PacketData() (uint8, []byte)
	UnpackData([]byte)
}

type Processor struct {
	msgInfo map[uint8]*MsgInfo
}

type MsgInfo struct {
	msgType       reflect.Type
	msgRouter     *chanrpc.Server
	msgHandler    MsgHandler
	msgRawHandler MsgHandler
}

type MsgHandler func([]interface{})

type MsgRaw struct {
	msgID      uint8
	msgRawData []byte
}

func NewProcessor() *Processor {
	p := new(Processor)
	p.msgInfo = make(map[uint8]*MsgInfo)
	return p
}

func (p *Processor) UsePacketMode() bool {
	return true
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) Register(msg interface{}) uint8 {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("packet message pointer required")
	}
	msgID := reflect.ValueOf(msg).Interface().(Packet).Protocol()
	if msgID <= 0 {
		log.Fatal("unnamed packet message")
	}
	if _, ok := p.msgInfo[msgID]; ok {
		log.Fatal("message %v is already registered", msgID)
	}

	i := new(MsgInfo)
	i.msgType = msgType
	p.msgInfo[msgID] = i
	return msgID
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRouter(msg interface{}, msgRouter *chanrpc.Server) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("packet message pointer required")
	}
	msgID := reflect.ValueOf(msg).Interface().(Packet).Protocol()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}
	i.msgRouter = msgRouter
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetHandler(msg interface{}, msgHandler MsgHandler) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("packet message pointer required")
	}
	msgID := reflect.ValueOf(msg).Interface().(Packet).Protocol()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}
	i.msgHandler = msgHandler
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRawHandler(msgID uint8, msgRawHandler MsgHandler) {
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}
	i.msgRawHandler = msgRawHandler
}

// goroutine safe
func (p *Processor) Route(msg interface{}, userData interface{}) error {
	// raw
	if msgRaw, ok := msg.(MsgRaw); ok {
		i, ok := p.msgInfo[msgRaw.msgID]
		if !ok {
			return fmt.Errorf("message %v not registered", msgRaw.msgID)
		}
		if i.msgRawHandler != nil {
			i.msgRawHandler([]interface{}{msgRaw.msgID, msgRaw.msgRawData, userData})
		}
		return nil
	}

	// packet
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return errors.New("packet message pointer required")
	}
	msgID := reflect.ValueOf(msg).Interface().(Packet).Protocol()
	i, ok := p.msgInfo[msgID]
	if !ok {
		return fmt.Errorf("message %v not registered", msgID)
	}
	if i.msgHandler != nil {
		i.msgHandler([]interface{}{msg, userData})
	}
	if i.msgRouter != nil {
		i.msgRouter.Go(msgType, msg, userData)
	}
	return nil
}

// goroutine safe
func (p *Processor) Unmarshal(data []byte) (rlt interface{}, err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Error("Packet Unmarshal '%x' error: %v", data, r)
			err = r.(error)
			return
		}
	}()
	bufProto := bytes.NewBuffer(data[:1])
	var proto uint8
	if network.LittleEndian {
		err = binary.Read(bufProto, binary.LittleEndian, &proto)
	} else {
		err = binary.Read(bufProto, binary.BigEndian, &proto)
	}
	if err != nil {
		return nil, err
	}

	i, ok := p.msgInfo[proto]
	if !ok {
		return nil, fmt.Errorf("message %v not registered", proto)
	}
	msgData := data[1:]
	// msg
	if i.msgRawHandler != nil {
		return MsgRaw{proto, msgData}, nil
	} else {
		msg := reflect.New(i.msgType.Elem()).Interface()
		msg.(Packet).UnpackData(msgData)
		return msg, nil
	}
	panic("bug")
}

// goroutine safe
func (p *Processor) Marshal(msg interface{}) (rlt [][]byte, err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Error("Packet Marshal '%x' error: %v", msg, r)
			err = r.(error)
			return
		}
	}()
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return nil, errors.New("packet message pointer required")
	}
	proto, data := msg.(Packet).PacketData()
	return [][]byte{[]byte{byte(proto)}, data}, nil
}
