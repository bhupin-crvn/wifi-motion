package sse

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2/log"
)

type Emitter struct {
	w     *bufio.Writer
	flush func() error
	m     string
}

func NewBufioEmitter(bw *bufio.Writer, m string) *Emitter {
	return &Emitter{w: bw, flush: bw.Flush, m: m}
}

func (e *Emitter) Send(ev DataByteEvent) error {
	if _, err := e.w.Write(ev.Format()); err != nil {
		return err
	}
	flushErr := e.flush()
	if flushErr != nil {
		fmt.Printf("Error while flushing: %v. Closing HTTP connection.\n", flushErr)
		return flushErr
	}
	return nil
}

func (e *Emitter) SendJSON(id, typ string, v any) *FlowError {
	b, err := json.Marshal(v)
	if err != nil {
		log.Errorf("Error marshalling the data %s", e.m)
		return NewFlowError(err, true)
	}
	if !json.Valid(b) {
		log.Errorf("Invalid JSON data: %s, retrying on next iteration", e.m)
		return NewFlowError(err, true)
	}
	fmtErr := e.Send(DataByteEvent{ID: id, Type: typ, Data: b})
	if fmtErr != nil {
		log.Error("Error writing to buffer")
		return NewFlowError(err, false)
	}
	return NewFlowError(nil, true)
}

func (e *Emitter) Heartbeat() error {
	if _, err := e.w.WriteString(": ping\n\n"); err != nil {
		return err
	}
	return e.flush()
}
