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
		InstanceID: instanceID,
		Instance:   instance,
		Target:     target,
		Master:     master,
		CheckPoint: regexp.MustCompile(`CHECK_POINT\|MODE=(\d+)\|PING=(\d+)ms\|POOL=(\d+)\|TCPS=(\d+)\|UDPS=(\d+)\|TCPRX=(\d+)\|TCPTX=(\d+)\|UDPRX=(\d+)\|UDPTX=(\d+)`),
	}
}

func (w *InstanceLogWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	scanner := bufio.NewScanner(strings.NewReader(s))

	for scanner.Scan() {
		line := scanner.Text()
		if matches := w.CheckPoint.FindStringSubmatch(line); len(matches) == 10 {
			if mode, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
				w.Instance.Mode = int32(mode)
			}
			if ping, err := strconv.ParseInt(matches[2], 10, 32); err == nil {
				w.Instance.Ping = int32(ping)
			}
			if pool, err := strconv.ParseInt(matches[3], 10, 32); err == nil {
				w.Instance.Pool = int32(pool)
			}
			if tcps, err := strconv.ParseInt(matches[4], 10, 32); err == nil {
				w.Instance.TCPS = int32(tcps)
			}
			if udps, err := strconv.ParseInt(matches[5], 10, 32); err == nil {
				w.Instance.UDPS = int32(udps)
			}

			stats := []*uint64{&w.Instance.TCPRX, &w.Instance.TCPTX, &w.Instance.UDPRX, &w.Instance.UDPTX}
			bases := []uint64{w.Instance.tcpRXBase, w.Instance.tcpTXBase, w.Instance.udpRXBase, w.Instance.udpTXBase}
			resets := []*uint64{&w.Instance.tcpRXReset, &w.Instance.tcpTXReset, &w.Instance.udpRXReset, &w.Instance.udpTXReset}
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

			w.Instance.lastCheckPoint = time.Now()

			if w.Instance.Status == "error" {
				w.Instance.Status = "running"
			}

			if !w.Instance.deleted {
				w.Master.Instances.Store(w.InstanceID, w.Instance)
				w.Master.SendSSEEvent("update", w.Instance)
			}
			continue
		}

		if w.Instance.Status != "error" && !w.Instance.deleted &&
			(strings.Contains(line, "Server error:") || strings.Contains(line, "Client error:")) {
			w.Instance.Status = "error"
			w.Instance.Ping = 0
			w.Instance.Pool = 0
			w.Instance.TCPS = 0
			w.Instance.UDPS = 0
			w.Master.Instances.Store(w.InstanceID, w.Instance)
		}

		fmt.Fprintf(w.Target, "%s [%s]\n", line, w.InstanceID)

		if !w.Instance.deleted {
			w.Master.SendSSEEvent("log", w.Instance, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(w.Target, "%s [%s]", s, w.InstanceID)
	}
	return len(p), nil
}

func (m *Master) HandleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	subscriberID := GenerateID()

	events := make(chan *InstanceEvent, 10)

	m.Subscribers.Store(subscriberID, events)
	defer m.Subscribers.Delete(subscriberID)

	fmt.Fprintf(w, "retry: %d\n\n", SSERetryTime)

	m.Instances.Range(func(_, value any) bool {
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
		if ch, exists := m.Subscribers.LoadAndDelete(subscriberID); exists {
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
				m.Logger.Error("HandleSSE: event marshal error: %v", err)
				continue
			}

			fmt.Fprintf(w, "event: instance\ndata: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (m *Master) SendSSEEvent(eventType string, instance *Instance, logs ...string) {
	event := &InstanceEvent{
		Type:     eventType,
		Time:     time.Now(),
		Instance: instance,
	}

	if len(logs) > 0 {
		event.Logs = logs[0]
	}

	select {
	case m.NotifyChannel <- event:
	default:
	}
}

func (m *Master) ShutdownSSEConnections() {
	var wg sync.WaitGroup

	m.Subscribers.Range(func(key, value any) bool {
		ch := value.(chan *InstanceEvent)
		wg.Add(1)
		go func(subscriberID any, eventChan chan *InstanceEvent) {
			defer wg.Done()
			select {
			case eventChan <- &InstanceEvent{Type: "shutdown", Time: time.Now()}:
			default:
			}
			if _, exists := m.Subscribers.LoadAndDelete(subscriberID); exists {
				close(eventChan)
			}
		}(key, ch)
		return true
	})

	wg.Wait()
}

func (m *Master) StartEventDispatcher() {
	for event := range m.NotifyChannel {
		m.Subscribers.Range(func(_, value any) bool {
			eventChan := value.(chan *InstanceEvent)
			select {
			case eventChan <- event:
			default:
			}
			return true
		})
	}
}
