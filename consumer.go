package eventsource

import (
	"bytes"
	"net"
	"net/http"
	"strconv"
	"time"
)

type consumer struct {
	conn   net.Conn
	es     *eventSource
	in     chan []byte
	staled bool
	lastId int
}

func newConsumer(resp http.ResponseWriter, req *http.Request, es *eventSource) (*consumer, error) {

	hid := req.Header.Get("Last-Event-ID")
	id := -1
	if len(hid) > 0 {
		var err error
		if id, err = strconv.Atoi(hid); err != nil {
			return nil, err
		}
	}

	conn, _, err := resp.(http.Hijacker).Hijack()
	if err != nil {
		return nil, err
	}

	consumer := &consumer{
		conn:   conn,
		es:     es,
		in:     make(chan []byte, 10),
		staled: false,
		lastId: id,
	}

	headers := [][]byte{
		[]byte("HTTP/1.1 200 OK"),
		[]byte("Content-Type: text/event-stream"),
	}

	if es.customHeadersFunc != nil {
		customHeaders := es.customHeadersFunc(req)
		headers = append(headers, customHeaders...)
	}

	headersData := append(bytes.Join(headers, []byte("\n")), []byte("\n\n")...)

	_, err = conn.Write(headersData)
	if err != nil {
		conn.Close()
		return nil, err
	}

	go func() {
		for message := range consumer.in {
			conn.SetWriteDeadline(time.Now().Add(consumer.es.timeout))
			_, err := conn.Write(message)
			if err != nil {
				netErr, ok := err.(net.Error)
				if !ok || !netErr.Timeout() || consumer.es.closeOnTimeout {
					consumer.staled = true
					consumer.conn.Close()
					consumer.es.staled <- consumer
					return
				}
			}
		}
		consumer.conn.Close()
	}()

	return consumer, nil
}
