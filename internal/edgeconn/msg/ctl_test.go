package msg

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestMsgWireRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	original := ClientHello{
		MachineID:   "mach_test",
		SoftVersion: "1.0.0",
		Timestamp:   1748245678,
		Nonce:       "nonce-1",
	}
	if err := WriteMsg(&buf, original); err != nil {
		t.Fatalf("WriteMsg returned error: %v", err)
	}
	if buf.Bytes()[0] != TypeClientHello {
		t.Fatalf("type byte = %q, want %q", buf.Bytes()[0], TypeClientHello)
	}

	decoded, err := ReadMsg(&buf)
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	hello, ok := decoded.(*ClientHello)
	if !ok {
		t.Fatalf("decoded type = %T, want *ClientHello", decoded)
	}
	if hello.MachineID != "mach_test" {
		t.Fatalf("MachineID = %q, want mach_test", hello.MachineID)
	}
}

func TestEmptyMessageWireSize(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteMsg(&buf, ReqWorkConn{}); err != nil {
		t.Fatalf("WriteMsg returned error: %v", err)
	}
	if buf.Len() != 1+8+2 {
		t.Fatalf("encoded len = %d, want 11", buf.Len())
	}
}

func TestPingRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ts := time.Now().Unix()
	if err := WriteMsg(&buf, Ping{Ts: ts}); err != nil {
		t.Fatalf("WriteMsg returned error: %v", err)
	}
	decoded, err := ReadMsg(&buf)
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	ping, ok := decoded.(*Ping)
	if !ok {
		t.Fatalf("decoded type = %T, want *Ping", decoded)
	}
	if ping.Ts != ts {
		t.Fatalf("Ts = %d, want %d", ping.Ts, ts)
	}
}

func TestReqWorkConnLengthPrefix(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteMsg(&buf, ReqWorkConn{}); err != nil {
		t.Fatalf("WriteMsg returned error: %v", err)
	}
	var length int64
	if err := binary.Read(bytes.NewReader(buf.Bytes()[1:9]), binary.BigEndian, &length); err != nil {
		t.Fatalf("read length: %v", err)
	}
	if length != 2 {
		t.Fatalf("payload length = %d, want 2", length)
	}
}
