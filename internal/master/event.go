package master

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func NewInstanceLogWriter(instanceID string, instance *Instance, target io.Writer, master *Master) *InstanceLogWriter {
	return &InstanceLogWriter{
		instanceID: instanceID,
		instance:   instance,
		target:     target,
		master:     master,
		checkPoint: regexp.MustCompile(`CHECK_POINT\|MODE=(\d+)\|PING=(\d+)ms\|POOL=(\d+)\|TCPS=(\d+)\|UDPS=(\d+)\|TCPRX=(\d+)\|TCPTX=(\d+)\|UDPRX=(\d+)\|UDPTX=(\d+)`),
	}
}

func (w *InstanceLogWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	scanner := bufio.NewScanner(strings.NewReader(s))

	for scanner.Scan() {
		line := scanner.Text()
		if matches := w.checkPoint.FindStringSubmatch(line); len(matches) == 10 {
			if mode, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
				w.instance.Mode = int32(mode)
			}
			if ping, err := strconv.ParseInt(matches[2], 10, 32); err == nil {
				w.instance.Ping = int32(ping)
			}
			if pool, err := strconv.ParseInt(matches[3], 10, 32); err == nil {
				w.instance.Pool = int32(pool)
			}
			if tcps, err := strconv.ParseInt(matches[4], 10, 32); err == nil {
				w.instance.TCPS = int32(tcps)
			}
			if udps, err := strconv.ParseInt(matches[5], 10, 32); err == nil {
				w.instance.UDPS = int32(udps)
			}

			stats := []*uint64{&w.instance.TCPRX, &w.instance.TCPTX, &w.instance.UDPRX, &w.instance.UDPTX}
			bases := []uint64{w.instance.tcpRXBase, w.instance.tcpTXBase, w.instance.udpRXBase, w.instance.udpTXBase}
			resets := []*uint64{&w.instance.tcpRXReset, &w.instance.tcpTXReset, &w.instance.udpRXReset, &w.instance.udpTXReset}
			for i, stat := range stats {
				if v, err := strconv.ParseUint(matches[i+6], 10, 64); err == nil {
					if v >= *resets[i] {
						*stat = bases[i] + v - *resets[i]
					} else {
						*stat = bases[i] + v
						*resets[i] = 0
					}
				}
			}

			w.instance.lastCheckPoint = time.Now()

			if w.instance.Status == "error" {
				w.instance.Status = "running"
			}

			if !w.instance.deleted {
				w.master.instances.Store(w.instanceID, w.instance)
				w.master.sendSSEEvent("update", w.instance)
			}
			continue
		}

		if w.instance.Status != "error" && !w.instance.deleted &&
			(strings.Contains(line, "Server error:") || strings.Contains(line, "Client error:")) {
			w.instance.Status = "error"
			w.instance.Ping = 0
			w.instance.Pool = 0
			w.instance.TCPS = 0
			w.instance.UDPS = 0
			w.master.instances.Store(w.instanceID, w.instance)
		}

		fmt.Fprintf(w.target, "%s [%s]\n", line, w.instanceID)

		if !w.instance.deleted {
			w.master.sendSSEEvent("log", w.instance, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(w.target, "%s [%s]", s, w.instanceID)
	}
	return len(p), nil
}

func (m *Master) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	subscriberID := generateID()

	events := make(chan *InstanceEvent, 10)

	m.subscribers.Store(subscriberID, events)
	defer m.subscribers.Delete(subscriberID)

	fmt.Fprintf(w, "retry: %d\n\n", sseRetryTime)

	m.instances.Range(func(_, value any) bool {
		instance := value.(*Instance)
		event := &InstanceEvent{
			Type:     "initial",
			Time:     time.Now(),
			Instance: instance,
		}

		data, err := json.Marshal(event)
		if err == nil {
			fmt.Fprintf(w, "event: instance\ndata: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		return true
	})

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	connectionClosed := make(chan struct{})

	go func() {
		<-ctx.Done()
		close(connectionClosed)
		if ch, exists := m.subscribers.LoadAndDelete(subscriberID); exists {
			close(ch.(chan *InstanceEvent))
		}
	}()

	for {
		select {
		case <-connectionClosed:
			return
		case event, ok := <-events:
			if !ok {
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				m.Logger.Error("handleSSE: event marshal error: %v", err)
				continue
			}

			fmt.Fprintf(w, "event: instance\ndata: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (m *Master) sendSSEEvent(eventType string, instance *Instance, logs ...string) {
	event := &InstanceEvent{
		Type:     eventType,
		Time:     time.Now(),
		Instance: instance,
	}

	if len(logs) > 0 {
		event.Logs = logs[0]
	}

	select {
	case m.notifyChannel <- event:
	default:
	}
}

func (m *Master) shutdownSSEConnections() {
	var wg sync.WaitGroup

	m.subscribers.Range(func(key, value any) bool {
		ch := value.(chan *InstanceEvent)
		wg.Add(1)
		go func(subscriberID any, eventChan chan *InstanceEvent) {
			defer wg.Done()
			select {
			case eventChan <- &InstanceEvent{Type: "shutdown", Time: time.Now()}:
			default:
			}
			if _, exists := m.subscribers.LoadAndDelete(subscriberID); exists {
				close(eventChan)
			}
		}(key, ch)
		return true
	})

	wg.Wait()
}

func (m *Master) startEventDispatcher() {
	for event := range m.notifyChannel {
		m.subscribers.Range(func(_, value any) bool {
			eventChan := value.(chan *InstanceEvent)
			select {
			case eventChan <- event:
			default:
			}
			return true
		})
	}
}
