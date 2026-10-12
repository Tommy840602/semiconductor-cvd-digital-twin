package main

import (
	"cvd/protocol/contract"
	"sync"
)

// Preserve MQTT order within each equipment while independent tools persist in parallel.
// A full queue applies backpressure; no event or birth is discarded.
func equipmentDispatcher(consume func(string, []byte)) func(string, []byte) {
	type message struct {
		topic string
		raw   []byte
	}
	var mu sync.Mutex
	queues := map[string]chan message{}
	return func(topic string, raw []byte) {
		kind, id, err := contract.ParseTopic(topic)
		if err != nil || kind == "NCMD" || kind == "DCMD" {
			return
		}
		mu.Lock()
		queue := queues[id]
		if queue == nil {
			queue = make(chan message, 32)
			queues[id] = queue
			go func(q <-chan message) {
				for m := range q {
					consume(m.topic, m.raw)
				}
			}(queue)
		}
		mu.Unlock()
		queue <- message{topic, append([]byte(nil), raw...)}
	}
}
