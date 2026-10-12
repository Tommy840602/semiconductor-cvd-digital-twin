package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type queuedDelivery struct {
	d        Delivery
	path     string
	sequence float64
	at       time.Time
	file     os.FileInfo
}

// Cache only ordering metadata. Bodies are read for the selected batch only.
// save invalidates replacements under the same lock; disk remains authoritative
// after restart and no record is removed until both required acknowledgements.
func (b *Bridge) deliveryBatch(limit int) ([]queuedDelivery, error) {
	b.outboxMu.Lock()
	defer b.outboxMu.Unlock()
	entries, err := os.ReadDir(filepath.Join(b.dir, "outbox"))
	if err != nil {
		return nil, err
	}
	if b.outboxIndex == nil {
		b.outboxIndex = make(map[string]queuedDelivery)
	}
	list := make([]queuedDelivery, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(b.dir, "outbox", entry.Name())
		q, ok := b.outboxIndex[path]
		if !ok {
			raw, e := os.ReadFile(path)
			if e != nil {
				return nil, e
			}
			var d Delivery
			if e = json.Unmarshal(raw, &d); e != nil {
				return nil, fmt.Errorf("corrupt bridge outbox %s: %w", entry.Name(), e)
			}
			var order struct {
				Sequence float64   `json:"sequence"`
				Version  float64   `json:"version"`
				Time     time.Time `json:"sourceTime"`
			}
			if e = json.Unmarshal(d.Body, &order); e != nil {
				return nil, e
			}
			if !d.Ack {
				order.Sequence = order.Version
			}
			info, e := os.Stat(path)
			if e != nil {
				return nil, e
			}
			d.Body = nil
			q = queuedDelivery{d, path, order.Sequence, order.Time, info}
			b.outboxIndex[path] = q
		}
		list = append(list, q)
	}
	previous := b.pending.Swap(int64(len(list)))
	if len(list) > 10000 && previous <= 10000 {
		log.Print("ALARM bridge outbox above 10000 records; durable backlog retained")
	}
	// Oldest first across equipment avoids starving later equipment IDs.
	sort.Slice(list, func(i, j int) bool {
		a, c := list[i], list[j]
		if !a.at.Equal(c.at) {
			return a.at.Before(c.at)
		}
		if a.d.Equipment != c.d.Equipment {
			return a.d.Equipment < c.d.Equipment
		}
		if a.d.Topic != c.d.Topic {
			return a.d.Topic < c.d.Topic
		}
		if a.sequence != c.sequence {
			return a.sequence < c.sequence
		}
		return a.path < c.path
	})
	if len(list) > limit {
		list = list[:limit]
	}
	for i := range list {
		raw, e := os.ReadFile(list[i].path)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &list[i].d); e != nil {
			return nil, e
		}
	}
	return list, nil
}

func (b *Bridge) removeDelivered(q queuedDelivery) error {
	b.outboxMu.Lock()
	defer b.outboxMu.Unlock()
	current, err := os.Stat(q.path)
	if os.IsNotExist(err) {
		delete(b.outboxIndex, q.path)
		return nil
	}
	if err != nil {
		return err
	}
	// Atomic rename can replace a snapshot while Kafka is acknowledging the old
	// copy. Keep the newer copy queued rather than deleting unsent data.
	if !os.SameFile(current, q.file) {
		return nil
	}
	if err = os.Remove(q.path); err == nil {
		delete(b.outboxIndex, q.path)
	}
	return err
}
