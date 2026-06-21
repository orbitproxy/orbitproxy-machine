package msg

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestDispatcherSendReceive(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	d := NewDispatcher(client)
	var got Message
	var wg sync.WaitGroup
	wg.Add(1)
	d.RegisterHandler(&Pong{}, func(m Message) {
		got = m
		wg.Done()
	})
	d.Run()

	if err := d.Send(Ping{Ts: 42}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	go func() {
		_, _ = ReadMsg(server)
		if err := WriteMsg(server, Pong{Ts: 42}); err != nil {
			t.Errorf("WriteMsg returned error: %v", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler was not invoked")
	}

	if got == nil {
		t.Fatal("expected pong message")
	}
	pong, ok := got.(*Pong)
	if !ok {
		t.Fatalf("got %T, want *Pong", got)
	}
	if pong.Ts != 42 {
		t.Fatalf("pong ts = %d, want 42", pong.Ts)
	}
}

func TestDispatcherSendAfterClose(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	d := NewDispatcher(client)
	d.Run()
	_ = server.Close()

	select {
	case <-d.Done():
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not exit after read error")
	}

	if err := d.Send(Ping{Ts: 1}); err != io.EOF {
		t.Fatalf("Send after close = %v, want EOF", err)
	}
}
