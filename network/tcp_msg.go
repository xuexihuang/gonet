package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// --------------
// | len | data |
// --------------
type MsgParser struct {
	lenMsgLen     int
	minMsgLen     uint32
	maxMsgLen     uint32
	usePacketMode bool
}

func NewMsgParser() *MsgParser {
	p := new(MsgParser)
	p.lenMsgLen = 2
	p.minMsgLen = 1
	p.maxMsgLen = 4096
	p.usePacketMode = false
	return p
}

// It's dangerous to call the method on reading or writing
func (p *MsgParser) SetMsgLen(lenMsgLen int, minMsgLen uint32, maxMsgLen uint32) {
	if lenMsgLen == 1 || lenMsgLen == 2 || lenMsgLen == 4 {
		p.lenMsgLen = lenMsgLen
	}
	if minMsgLen != 0 {
		p.minMsgLen = minMsgLen
	}
	if maxMsgLen != 0 {
		p.maxMsgLen = maxMsgLen
	}

	var max uint32
	switch p.lenMsgLen {
	case 1:
		max = math.MaxUint8
	case 2:
		max = math.MaxUint16
	case 3:
		max = math.MaxUint16
	case 4:
		max = math.MaxUint32
	}
	if p.minMsgLen > max {
		p.minMsgLen = max
	}
	if p.maxMsgLen > max {
		p.maxMsgLen = max
	}
}

// It's dangerous to call the method on reading or writing
func (p *MsgParser) SetPacketMode(usePacketMode bool) {
	p.usePacketMode = usePacketMode
}

// goroutine safe
func (p *MsgParser) Read(conn *TCPConn) ([]byte, error) {
	if p.usePacketMode {
		return p.ReadPacket(conn)
	}
	var b [4]byte
	bufMsgLen := b[:p.lenMsgLen]

	// read len
	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return nil, err
	}

	// parse len
	var msgLen uint32
	switch p.lenMsgLen {
	case 1:
		msgLen = uint32(bufMsgLen[0])
	case 2:
		if LittleEndian {
			msgLen = uint32(binary.LittleEndian.Uint16(bufMsgLen))
		} else {
			msgLen = uint32(binary.BigEndian.Uint16(bufMsgLen))
		}
	case 4:
		if LittleEndian {
			msgLen = binary.LittleEndian.Uint32(bufMsgLen)
		} else {
			msgLen = binary.BigEndian.Uint32(bufMsgLen)
		}
	}

	// check len
	if msgLen > p.maxMsgLen {
		return nil, errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return nil, errors.New(fmt.Sprintf("message too short: %d", msgLen))
	}

	// data
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, err
	}

	return msgData, nil
}

// goroutine safe
func (p *MsgParser) Write(conn *TCPConn, args ...[]byte) error {
	if p.usePacketMode {
		return p.WritePacket(conn, args...)
	}
	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	// check len
	if msgLen > p.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return errors.New("message too short")
	}

	msg := make([]byte, uint32(p.lenMsgLen)+msgLen)

	// write len
	switch p.lenMsgLen {
	case 1:
		msg[0] = byte(msgLen)
	case 2:
		if LittleEndian {
			binary.LittleEndian.PutUint16(msg, uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg, uint16(msgLen))
		}
	case 4:
		if LittleEndian {
			binary.LittleEndian.PutUint32(msg, msgLen)
		} else {
			binary.BigEndian.PutUint32(msg, msgLen)
		}
	}

	// write data
	l := p.lenMsgLen
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	conn.Write(msg)

	return nil
}

// goroutine safe
// 压解包模式下，包实际头部长度要比定义的包长度多1（在最开始，用于封装协议号或者其他）。
func (p *MsgParser) ReadPacket(conn *TCPConn) ([]byte, error) {
	headerSize := p.lenMsgLen + 1
	bufMsgLen := make([]byte, headerSize)
	// read len
	_, err := io.ReadFull(conn, bufMsgLen)
	if err != nil {
		return nil, err
	}
	var dataLen uint16 = 0
	bufDataLen := bytes.NewBuffer(bufMsgLen[1:])
	//fmt.Printf("littleEndian: %v bufDataLen000: %x  dataLen: %d\n", LittleEndian, bufDataLen, dataLen)
	if LittleEndian {
		err = binary.Read(bufDataLen, binary.LittleEndian, &dataLen)
	} else {
		err = binary.Read(bufDataLen, binary.BigEndian, &dataLen)
	}
	//fmt.Printf("bufDataLen: %x  dataLen: %d err: %v\n", bufDataLen, dataLen, err)
	if err != nil {
		return nil, err
	}
	if uint32(dataLen) > p.maxMsgLen {
		return nil, errors.New("message too long")
	}
	// data
	msgData := make([]byte, uint32(dataLen)+1)
	// proto
	copy(msgData[:1], bufMsgLen[:1])
	if _, err := io.ReadFull(conn, msgData[1:]); err != nil {
		return nil, err
	}
	return msgData, nil
}

// goroutine safe
func (p *MsgParser) WritePacket(conn *TCPConn, args ...[]byte) error {
	var msgLen uint32 = 0
	bufProto := args[0]
	l := uint32(len(bufProto))
	if l != 1 {
		return errors.New("Packet Mode, Fist Bytes Len != 1")
	}
	for i := 1; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}
	// write proto
	msg := make([]byte, msgLen+l+uint32(p.lenMsgLen))
	copy(msg, bufProto)
	// write len
	switch p.lenMsgLen {
	case 1:
		msg[l] = byte(msgLen)
	case 2:
		if LittleEndian {
			binary.LittleEndian.PutUint16(msg[l:], uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg[l:], uint16(msgLen))
		}
	case 4:
		if LittleEndian {
			binary.LittleEndian.PutUint32(msg[l:], msgLen)
		} else {
			binary.BigEndian.PutUint32(msg[l:], msgLen)
		}
	}
	// write data
	l += uint32(p.lenMsgLen)
	for i := 1; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += uint32(len(args[i]))
	}
	conn.Write(msg)
	return nil
}
