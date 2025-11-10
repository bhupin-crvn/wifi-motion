package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

const unexpectedOutput = "unexpected output:\n--- got ---\n%q\n--- want ---\n%q"

// helper to build an Emitter that writes into a bytes.Buffer
func newTestEmitter() (*Emitter, *bytes.Buffer) {
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	em := &Emitter{
		w:     bw,
		flush: func() error { return bw.Flush() },
	}
	return em, &buf
}

func TestSendEventWithAllFields(t *testing.T) {
	em, buf := newTestEmitter()

	ev := DataByteEvent{
		ID:    "789",
		Type:  "long-message",
		Retry: 1500 * time.Millisecond,
		Data:  []byte(`{"ok":true}`),
	}
	if err := em.Send(ev); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	got := buf.String()
	want := "" +
		"id: 789\n" +
		"event: long-message\n" +
		"retry: 1500\n" +
		"data: {\"ok\":true}\n\n"

	if got != want {
		t.Fatalf(unexpectedOutput, got, want)
	}
}

func TestSendEventMinimal(t *testing.T) {
	em, buf := newTestEmitter()

	ev := DataByteEvent{Data: []byte(`{"msg":"hi"}`)}
	if err := em.Send(ev); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	got := buf.String()
	want := "data: {\"msg\":\"hi\"}\n\n"
	if got != want {
		t.Fatalf(unexpectedOutput, got, want)
	}
}

func TestSendJSON(t *testing.T) {
	em, buf := newTestEmitter()

	type payload struct {
		Step int `json:"step"`
	}
	if flowErr := em.SendJSON("42", "progress", payload{Step: 7}); flowErr.Err != nil {
		t.Fatalf("SendJSON returned error: %v", flowErr.Err)
	}

	got := buf.String()

	// Build expected JSON deterministically
	wantJSON, _ := json.Marshal(payload{Step: 7})
	want := "" +
		"id: 42\n" +
		"event: progress\n" +
		"data: " + string(wantJSON) + "\n\n"

	if got != want {
		t.Fatalf(unexpectedOutput, got, want)
	}
}

func TestHeartbeatHelper(t *testing.T) {
	em, buf := newTestEmitter()

	if err := em.Heartbeat(); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	got := buf.String()
	want := ":\n\n"
	if got != want {
		t.Fatalf(unexpectedOutput, got, want)
	}
}

func TestMultipleEvents(t *testing.T) {
	em, buf := newTestEmitter()

	// first
	if err := em.Send(DataByteEvent{ID: "1", Type: "tick", Data: []byte(`"a"`)}); err != nil {
		t.Fatalf("Send #1 error: %v", err)
	}
	// second
	if err := em.Send(DataByteEvent{ID: "2", Type: "tick", Data: []byte(`"b"`)}); err != nil {
		t.Fatalf("Send #2 error: %v", err)
	}
	// heartbeat
	if err := em.Heartbeat(); err != nil {
		t.Fatalf("Heartbeat error: %v", err)
	}

	got := buf.String()
	want := "" +
		"id: 1\nevent: tick\ndata: \"a\"\n\n" +
		"id: 2\nevent: tick\ndata: \"b\"\n\n" +
		":\n\n"

	if got != want {
		t.Fatalf(unexpectedOutput, got, want)
	}
}
