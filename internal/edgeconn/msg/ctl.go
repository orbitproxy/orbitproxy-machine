package msg

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

const defaultMaxMsgLength int64 = 10240

var (
	ErrMsgType      = errors.New("message type error")
	ErrMaxMsgLength = errors.New("message length exceed the limit")
	ErrMsgLength    = errors.New("message length error")
	ErrMsgFormat    = errors.New("message format error")
)

type msgCtl struct {
	typeMap      map[byte]reflect.Type
	typeByteMap  map[reflect.Type]byte
	maxMsgLength int64
}

var ctl *msgCtl

func init() {
	ctl = &msgCtl{
		typeMap:      make(map[byte]reflect.Type),
		typeByteMap:  make(map[reflect.Type]byte),
		maxMsgLength: defaultMaxMsgLength,
	}
	registerMsg(TypeClientHello, ClientHello{})
	registerMsg(TypeServerHello, ServerHello{})
	registerMsg(TypePing, Ping{})
	registerMsg(TypePong, Pong{})
	registerMsg(TypeReqWorkConn, ReqWorkConn{})
	registerMsg(TypeNewWorkConn, NewWorkConn{})
	registerMsg(TypeStartWorkConn, StartWorkConn{})
}

func registerMsg(typeByte byte, prototype Message) {
	t := reflect.TypeOf(prototype)
	ctl.typeMap[typeByte] = t
	ctl.typeByteMap[t] = typeByte
}

func ReadMsg(r io.Reader) (Message, error) {
	typeByte, payload, err := ctl.readFrame(r)
	if err != nil {
		return nil, err
	}
	return ctl.decode(typeByte, payload)
}

func WriteMsg(w io.Writer, m Message) error {
	frame, err := ctl.encode(m)
	if err != nil {
		return err
	}
	_, err = w.Write(frame)
	return err
}

func (c *msgCtl) readFrame(r io.Reader) (typeByte byte, payload []byte, err error) {
	if err = binary.Read(r, binary.BigEndian, &typeByte); err != nil {
		return 0, nil, fmt.Errorf("read msg type: %w", err)
	}
	if _, ok := c.typeMap[typeByte]; !ok {
		return 0, nil, ErrMsgType
	}

	var length int64
	if err = binary.Read(r, binary.BigEndian, &length); err != nil {
		return 0, nil, fmt.Errorf("read msg length: %w", err)
	}
	if length > c.maxMsgLength {
		return 0, nil, ErrMaxMsgLength
	}
	if length < 0 {
		return 0, nil, ErrMsgLength
	}

	payload = make([]byte, length)
	n, err := io.ReadFull(r, payload)
	if err != nil {
		return 0, nil, fmt.Errorf("read msg payload: %w", err)
	}
	if int64(n) != length {
		return 0, nil, ErrMsgFormat
	}
	return typeByte, payload, nil
}

func (c *msgCtl) decode(typeByte byte, payload []byte) (Message, error) {
	t, ok := c.typeMap[typeByte]
	if !ok {
		return nil, ErrMsgType
	}
	m := reflect.New(t).Interface().(Message)
	if len(payload) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(payload, m); err != nil {
		msgType := byteTypeMap[typeByte]
		return nil, fmt.Errorf("decode msg %q: %w", msgType, err)
	}
	return m, nil
}

func (c *msgCtl) encode(m Message) ([]byte, error) {
	typeByte, err := c.typeByteFor(m)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal msg: %w", err)
	}
	buf := bytes.NewBuffer(make([]byte, 0, 1+8+len(content)))
	if err := buf.WriteByte(typeByte); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, int64(len(content))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(content); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *msgCtl) typeByteFor(m Message) (byte, error) {
	t := reflect.TypeOf(m)
	if t == nil {
		return 0, fmt.Errorf("nil message")
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	typeByte, ok := c.typeByteMap[t]
	if !ok {
		return 0, fmt.Errorf("unknown message type %s", t.String())
	}
	return typeByte, nil
}

type unknownMessageTypeError struct {
	msgType MessageType
}

func (e unknownMessageTypeError) Error() string {
	return fmt.Sprintf("unknown message type: %q", e.msgType)
}

func errUnknownMessageType(msgType MessageType) error {
	return unknownMessageTypeError{msgType: msgType}
}
