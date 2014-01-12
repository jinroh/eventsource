package eventsource

import (
	"bytes"
	"fmt"
	"net/http"
)

const (
	consumerBufSize    = 10
	defaultHistorySize = 50
)

type consumer struct {
	id int
	ch chan []byte
}

type message struct {
	event string
	data  []byte
}

type eventSource struct {
	// History of last events.
	H [][]byte

	// Value of last event's id sent.
	id        int
	sink      chan *message
	add       chan *consumer
	remove    chan *consumer
	close     chan bool
	consumers map[*consumer]bool
}

// EventSource interface provides methods for sending messages
// and closing all connections.
type EventSource interface {
	// It should implement ServerHTTP method.
	http.Handler

	// Send message to all consumers.
	SendMessage(data []byte, event string)

	// Curried version of SendMessage, bound
	// to a unique event namespace
	Sender(event string) func(data []byte)

	// Consumers count
	ConsumersCount() int

	// Close and clear all consumers.
	Close()

	// Subscribe a new consumer to eventsource
	// wth the given last received event id (or -1)
	Subscribe(id int) *consumer

	// Unsubscribe the given consumer
	Unsubscribe(c *consumer)
}

func newMessage(data []byte, event string) *message {
	m := &message{
		event: event,
		data:  make([]byte, len(data)),
	}
	copy(m.data, data)
	return m
}

func (m *message) prepare(id int) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "id: %d\n", id)
	fmt.Fprintf(&buf, "event: %s\n", m.event)
	fmt.Fprintf(&buf, "data: %s\n\n", m.data)
	return buf.Bytes()
}

func (es *eventSource) loop() {
	for {
		select {

		// New message from the sink, broadcasted to consumers.
		case m := <-es.sink:
			b := m.prepare(es.id)
			for c := range es.consumers {
				select {
				case c.ch <- b:
				default:
				}
			}
			es.appendHistory(b)
			es.id++

		// New consumer added to consumer list.
		// Send missed events to the consumer if any.
		case c := <-es.add:
			es.consumers[c] = true
			for _, b := range es.missedEvents(c) {
				select {
				case c.ch <- b:
				default:
				}
			}

		// Remove one consumer.
		case c := <-es.remove:
			delete(es.consumers, c)
			close(c.ch)

		// Close eventsource's channels and consumers.
		case <-es.close:
			close(es.sink)
			close(es.add)
			close(es.remove)
			close(es.close)
			for c := range es.consumers {
				close(c.ch)
			}
			es.consumers = nil
			es.H = nil
			return
		}
	}
}

// New creates new EventSource instance.
func New() EventSource {
	es := &eventSource{
		H:         make([][]byte, 0, defaultHistorySize),
		id:        0,
		sink:      make(chan *message, 1),
		add:       make(chan *consumer),
		remove:    make(chan *consumer, 1),
		close:     make(chan bool),
		consumers: make(map[*consumer]bool),
	}
	go es.loop()
	return es
}

func (es *eventSource) Subscribe(id int) *consumer {
	c := &consumer{
		id: id,
		ch: make(chan []byte, consumerBufSize),
	}
	es.add <- c
	return c
}

func (es *eventSource) Unsubscribe(c *consumer) {
	es.remove <- c
}

func (es *eventSource) Close() {
	es.close <- true
}

func (es *eventSource) SendMessage(data []byte, event string) {
	es.sink <- newMessage(data, event)
}

func (es *eventSource) ConsumersCount() int {
	return len(es.consumers)
}

func (es *eventSource) Sender(event string) func(data []byte) {
	return func(data []byte) {
		es.SendMessage(data, event)
	}
}

// ServeHTTP implements http.Handler interface.
func (es *eventSource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	consumerHandler(w, r, es)
}

func (es *eventSource) appendHistory(b []byte) {
	if len(es.H) < defaultHistorySize {
		es.H = append(es.H, b)
	} else {
		es.H = append(es.H[1:], b)
	}
}

func (es *eventSource) missedEvents(c *consumer) [][]byte {
	if c.id < 0 || c.id >= es.id {
		return nil
	}
	i := c.id - es.id + len(es.H)
	if i < 0 {
		i = 0
	}
	return es.H[i:]
}
