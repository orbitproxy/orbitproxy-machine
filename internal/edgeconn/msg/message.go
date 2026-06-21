package msg

import "reflect"

type MessageType string

const (
	MessageTypeClientHello   MessageType = "client_hello"
	MessageTypeServerHello   MessageType = "server_hello"
	MessageTypePing          MessageType = "ping"
	MessageTypePong          MessageType = "pong"
	MessageTypeReqWorkConn   MessageType = "req_work_conn"
	MessageTypeNewWorkConn   MessageType = "new_work_conn"
	MessageTypeStartWorkConn MessageType = "start_work_conn"
)

const (
	TypeClientHello   byte = 'o'
	TypeServerHello   byte = '1'
	TypePing          byte = 'h'
	TypePong          byte = '4'
	TypeReqWorkConn   byte = 'r'
	TypeNewWorkConn   byte = 'w'
	TypeStartWorkConn byte = 's'
)

type Message interface {
	MsgType() MessageType
}

var (
	msgTypeMap = map[MessageType]Message{
		MessageTypeClientHello:   ClientHello{},
		MessageTypeServerHello:   ServerHello{},
		MessageTypePing:          Ping{},
		MessageTypePong:          Pong{},
		MessageTypeReqWorkConn:   ReqWorkConn{},
		MessageTypeNewWorkConn:   NewWorkConn{},
		MessageTypeStartWorkConn: StartWorkConn{},
	}
	byteTypeMap = map[byte]MessageType{
		TypeClientHello:   MessageTypeClientHello,
		TypeServerHello:   MessageTypeServerHello,
		TypePing:          MessageTypePing,
		TypePong:          MessageTypePong,
		TypeReqWorkConn:   MessageTypeReqWorkConn,
		TypeNewWorkConn:   MessageTypeNewWorkConn,
		TypeStartWorkConn: MessageTypeStartWorkConn,
	}
)

func newMessage(msgType MessageType) (Message, error) {
	prototype, ok := msgTypeMap[msgType]
	if !ok {
		return nil, errUnknownMessageType(msgType)
	}
	return reflect.New(reflect.TypeOf(prototype)).Interface().(Message), nil
}
