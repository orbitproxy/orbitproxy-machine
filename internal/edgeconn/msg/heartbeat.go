package msg

type Ping struct {
	Ts int64 `json:"ts"`
}

func (Ping) MsgType() MessageType { return MessageTypePing }

type Pong struct {
	Ts int64 `json:"ts"`
}

func (Pong) MsgType() MessageType { return MessageTypePong }
