package msg

import (
	"io"
	"reflect"
)

// AsyncHandler runs the handler in a new goroutine so readLoop is not blocked.
func AsyncHandler(f func(Message)) func(Message) {
	return func(m Message) {
		go f(m)
	}
}

// Dispatcher serializes writes and dispatches reads on a control connection
// (对齐 frp/pkg/msg.Dispatcher).
type Dispatcher struct {
	rw io.ReadWriter

	sendCh         chan Message
	doneCh         chan struct{}
	msgHandlers    map[reflect.Type]func(Message)
	defaultHandler func(Message)
}

func NewDispatcher(rw io.ReadWriter) *Dispatcher {
	return &Dispatcher{
		rw:          rw,
		sendCh:      make(chan Message, 100),
		doneCh:      make(chan struct{}),
		msgHandlers: make(map[reflect.Type]func(Message)),
	}
}

func (d *Dispatcher) Run() {
	go d.sendLoop()
	go d.readLoop()
}

func (d *Dispatcher) sendLoop() {
	for {
		select {
		case <-d.doneCh:
			return
		case m := <-d.sendCh:
			if err := WriteMsg(d.rw, m); err != nil {
				d.closeDone()
				return
			}
		}
	}
}

func (d *Dispatcher) readLoop() {
	for {
		m, err := ReadMsg(d.rw)
		if err != nil {
			d.closeDone()
			return
		}

		if handler, ok := d.msgHandlers[reflect.TypeOf(m)]; ok {
			handler(m)
		} else if d.defaultHandler != nil {
			d.defaultHandler(m)
		}
	}
}

func (d *Dispatcher) Send(m Message) error {
	select {
	case <-d.doneCh:
		return io.EOF
	case d.sendCh <- m:
		return nil
	}
}

func (d *Dispatcher) RegisterHandler(prototype Message, handler func(Message)) {
	d.msgHandlers[reflect.TypeOf(prototype)] = handler
}

func (d *Dispatcher) SetDefaultHandler(handler func(Message)) {
	d.defaultHandler = handler
}

func (d *Dispatcher) Done() <-chan struct{} {
	return d.doneCh
}

func (d *Dispatcher) closeDone() {
	select {
	case <-d.doneCh:
	default:
		close(d.doneCh)
	}
}
