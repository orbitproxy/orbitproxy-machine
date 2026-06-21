package msg

type ClientHello struct {
	MachineID     string `json:"machine_id"`
	SoftVersion   string `json:"soft_version"`
	Timestamp     int64  `json:"timestamp"`
	Nonce         string `json:"nonce"`
	AuthSignature string `json:"auth_signature"`
}

func (ClientHello) MsgType() MessageType { return MessageTypeClientHello }

type ServerHello struct {
	EdgeID    string `json:"edge_id"`
	SessionID string `json:"session_id"`
}

func (ServerHello) MsgType() MessageType { return MessageTypeServerHello }
