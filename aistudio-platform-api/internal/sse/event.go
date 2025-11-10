package sse

import (
	"bytes"
	"fmt"
	"time"
)

type DataByteEvent struct {
	ID    string
	Type  string
	Retry time.Duration
	Data  []byte
}

func (ev DataByteEvent) Format() []byte {
	var b bytes.Buffer

	if ev.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", ev.ID)
	}
	if ev.Type != "" {
		fmt.Fprintf(&b, "event: %s\n", ev.Type)
	}
	if ev.Retry > 0 {
		fmt.Fprintf(&b, "retry: %d\n", int(ev.Retry/time.Millisecond))
	}
	b.WriteString("data: ")
	b.Write(ev.Data)
	b.WriteString("\n\n")

	return b.Bytes()
}
