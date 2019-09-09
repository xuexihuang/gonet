package network

import (
	"github.com/xuexihuang/gonet/log"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
)

const (
	packetHeaderSize = 3
)

func UnpackProto(conn net.Conn) (uint8, []byte, error) {
	bufMsgLen := make([]byte, packetHeaderSize)
	// read len
	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return 0, nil, err
	}
	var proto uint8 = 0
	var err error
	bufProto := bytes.NewBuffer(bufMsgLen[:1])
	if LittleEndian {
		err = binary.Read(bufProto, binary.LittleEndian, &proto)
	} else {
		err = binary.Read(bufProto, binary.BigEndian, &proto)
	}
	if err != nil {
		return 0, nil, err
	}
	var dataLen uint16 = 0
	bufDataLen := bytes.NewBuffer(bufMsgLen[1:])
	if LittleEndian {
		err = binary.Read(bufDataLen, binary.LittleEndian, &dataLen)
	} else {
		err = binary.Read(bufDataLen, binary.BigEndian, &dataLen)
	}
	if err != nil {
		return 0, nil, err
	}

	// data
	msgData := make([]byte, int(dataLen))
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return 0, nil, err
	}
	return proto, msgData, nil
}

func PacketProto(proto uint8, msgData []byte) []byte {
	size := len(msgData)
	msg := make([]byte, size+packetHeaderSize)
	msg[0] = byte(proto)
	if LittleEndian {
		binary.LittleEndian.PutUint16(msg[1:], uint16(size))
	} else {
		binary.BigEndian.PutUint16(msg[1:], uint16(size))
	}
	copy(msg[packetHeaderSize:], msgData)
	return msg
}

func PacketSend(conn net.Conn, proto uint8, msgData []byte) {
	msg := PacketProto(proto, msgData)
	conn.Write(msg)
}

func PacketInt(to []byte, value int, size int) []byte {
	buf := make([]byte, int(size))
	switch size {
	case 1:
		if value > math.MaxUint8 {
			log.Error("PacketInt(1) %d >= math.MaxUint8", value)
			value = math.MaxUint8
		}
		buf[0] = uint8(value)
	case 2:
		if value > math.MaxUint16 {
			log.Error("PacketInt(2) %d >= math.MaxUint16", value)
			value = math.MaxUint16
		}
		if LittleEndian {
			binary.LittleEndian.PutUint16(buf, uint16(value))
		} else {
			binary.BigEndian.PutUint16(buf, uint16(value))
		}
	case 4:
		if value > math.MaxUint32 {
			log.Error("PacketInt(4) %d >= math.MaxUint32", value)
			value = math.MaxUint32
		}
		if LittleEndian {
			binary.LittleEndian.PutUint32(buf, uint32(value))
		} else {
			binary.BigEndian.PutUint32(buf, uint32(value))
		}
	case 8:
		if value > math.MaxInt64 {
			log.Error("PacketInt(8) %d >= math.MaxInt64", value)
			value = math.MaxInt64
		}
		if LittleEndian {
			binary.LittleEndian.PutUint64(buf, uint64(value))
		} else {
			binary.BigEndian.PutUint64(buf, uint64(value))
		}
	default:
		log.Error("Unknown PacketInt Size: %d", size)
		return buf
	}
	newBuf := to
	oldSize := len(to)
	if cap(to) < len(to)+len(buf) {
		newSize := len(to) + len(buf)
		newBuf = make([]byte, newSize)
		copy(newBuf, to)
	}
	copy(newBuf[oldSize:], buf)
	return newBuf
}

func UnpackInt(from []byte, size int) (int, []byte) {
	bufUnpack, bufLeft := UnpackBytes(from, size)
	if len(bufUnpack) <= 0 {
		return 0, []byte{}
	}
	buf := bytes.NewBuffer(bufUnpack)
	var value int = 0
	var err error
	switch size {
	case 1:
		var value2 uint8 = 0
		if LittleEndian {
			err = binary.Read(buf, binary.LittleEndian, &value2)
		} else {
			err = binary.Read(buf, binary.BigEndian, &value2)
		}
		value = int(value2)
	case 2:
		var value2 uint16 = 0
		if LittleEndian {
			err = binary.Read(buf, binary.LittleEndian, &value2)
		} else {
			err = binary.Read(buf, binary.BigEndian, &value2)
		}
		value = int(value2)
	case 4:
		var value2 uint32 = 0
		if LittleEndian {
			err = binary.Read(buf, binary.LittleEndian, &value2)
		} else {
			err = binary.Read(buf, binary.BigEndian, &value2)
		}
		value = int(value2)
	case 8:
		var value2 uint64 = 0
		if LittleEndian {
			err = binary.Read(buf, binary.LittleEndian, &value2)
		} else {
			err = binary.Read(buf, binary.BigEndian, &value2)
		}
		value = int(value2)
	default:
		log.Error("Unknown UnpackInt Size: %d", size)
	}
	if err != nil {
		panic(err)
	}
	return value, bufLeft
}

func PacketBytes(to []byte, from []byte, size int) []byte {
	newBuf := to
	oldSize := len(to)
	if size <= 0 {
		size = len(from)
	}
	newSize := len(to) + int(size)
	if cap(to) < newSize {
		newSize := newSize
		newBuf = make([]byte, newSize)
		copy(newBuf, to)
	}
	copy(newBuf[oldSize:], from[:size])
	return newBuf
}

func PacketString(to []byte, from string, size int) []byte {
	buf := []byte(from)
	return PacketBytes(to, buf, size)
}

func UnpackBytes(from []byte, size int) ([]byte, []byte) {
	if size <= 0 {
		return from, []byte{}
	}
	if len(from) < size {
		size = len(from)
	}
	buf := from[:size]
	bufLeft := from[size:]
	return buf, bufLeft
}

func UnpackString(from []byte, size int) (string, []byte) {
	buf, bufLeft := UnpackBytes(from, size)
	n := bytes.Index(buf, []byte{0})
	if n >= 0 {
		return string(buf[:n]), bufLeft
	}
	return string(buf), bufLeft
}
