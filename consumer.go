package eventsource

import (
	"log"
	"net/http"
	"strconv"
)

func lastEventId(r *http.Request) (int, error) {
	h := r.Header.Get("Last-Event-ID")
	if len(h) > 0 {
		return strconv.Atoi(h)
	} else {
		return -1, nil
	}
}

func consumerHandler(w http.ResponseWriter, r *http.Request, es *eventSource) {
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(w, "eventsource: should accept text/event-stream", http.StatusNotAcceptable)
		return
	}

	var (
		fl  http.Flusher
		con *consumer
		id  int
		err error
		ok  bool
	)

	id, err = lastEventId(r)
	if err != nil {
		http.Error(w, "eventsource: bad last-event-id header", http.StatusBadRequest)
		return
	}

	fl, ok = w.(http.Flusher)
	if !ok {
		http.Error(w, "eventsouce: response does not implement http.Flusher", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")

	con = es.Subscribe(id)
	defer es.Unsubscribe(con)

	for {
		b := <-con.ch
		if _, err := w.Write(b); err == nil {
			fl.Flush()
		} else {
			log.Print(err)
			return
		}
	}
}
