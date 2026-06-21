package msg

import "encoding/json"

type ReqWorkConn struct{}

func (ReqWorkConn) MsgType() MessageType { return MessageTypeReqWorkConn }

type NewWorkConn struct {
	SessionID string `json:"session_id"`
	MachineID string `json:"machine_id"`
}

func (NewWorkConn) MsgType() MessageType { return MessageTypeNewWorkConn }

type StartWorkConn struct {
	ProxyID              string          `json:"proxy_id"`
	EndpointID           string          `json:"endpoint_id"`
	RouteVersion         string          `json:"route_version"`
	SourceAddr           string          `json:"source_addr"`
	LocalIP              string          `json:"local_ip"`
	LocalPort            int             `json:"local_port"`
	EndpointPluginConfig json.RawMessage `json:"endpoint_plugin_config,omitempty"`
	Error                string          `json:"error,omitempty"`
}

func (StartWorkConn) MsgType() MessageType { return MessageTypeStartWorkConn }
