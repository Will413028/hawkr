package sse

import (
	"encoding/json"
	"sync"
	"time"
)

// batchAggregator buffers events for a single topic over a sliding window and
// emits them as either the original event (size==1) or a "<channel>.batch"
// merged event (size>1). 设计书 §5.7。
//
// Concurrency: add() serialized via mu; one timer goroutine flushes via Hub.flushBatch.
type batchAggregator struct {
	topic          string
	batchEventName string
	window         time.Duration
	maxEvents      int
	hub            *Hub

	mu    sync.Mutex
	buf   []BufferedEvent
	timer *time.Timer
}

func newBatchAggregator(topic, batchEventName string, window time.Duration, maxEvents int, hub *Hub) *batchAggregator {
	if maxEvents <= 0 {
		maxEvents = 100
	}
	if window <= 0 {
		window = 50 * time.Millisecond
	}
	return &batchAggregator{
		topic:          topic,
		batchEventName: batchEventName,
		window:         window,
		maxEvents:      maxEvents,
		hub:            hub,
	}
}

// add appends an event. If the window has not started, starts it.
// If buffer reaches maxEvents, flushes immediately.
func (a *batchAggregator) add(ev BufferedEvent) {
	a.mu.Lock()
	a.buf = append(a.buf, ev)
	shouldFlush := len(a.buf) >= a.maxEvents
	if a.timer == nil {
		a.timer = time.AfterFunc(a.window, a.onTimer)
	}
	a.mu.Unlock()
	if shouldFlush {
		a.flush()
	}
}

func (a *batchAggregator) onTimer() {
	a.flush()
}

func (a *batchAggregator) flush() {
	a.mu.Lock()
	events := a.buf
	a.buf = nil
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.mu.Unlock()

	if len(events) > 0 {
		a.hub.flushBatch(a.topic, a.batchEventName, events)
	}
}

// batchPayload is the JSON shape of a "<channel>.batch" event data field.
// 设计书 §3.5 / §5.7：每筆元素含原 event name + 原 data + id.
type batchPayload struct {
	Events []batchPayloadEntry `json:"events"`
}

type batchPayloadEntry struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
	ID    string          `json:"id"`
}

// buildBatchData marshals N events into a single batch payload data byte slice。
func buildBatchData(events []BufferedEvent) []byte {
	entries := make([]batchPayloadEntry, 0, len(events))
	for _, e := range events {
		entries = append(entries, batchPayloadEntry{
			Event: e.EventName,
			Data:  json.RawMessage(e.Data),
			ID:    e.ID,
		})
	}
	out, err := json.Marshal(batchPayload{Events: entries})
	if err != nil {
		// Should not happen — RawMessage is opaque. Return an empty array on failure。
		return []byte(`{"events":[]}`)
	}
	return out
}
