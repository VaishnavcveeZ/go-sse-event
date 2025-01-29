package sseevent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
)

var (
	store *eventStore
)

type (
	eventStore struct {
		events map[string]*event
	}

	event struct {
		id          string
		stream      chan eventResponseData
		connections map[string]chan eventResponseData
	}

	listener struct {
		ctx     context.Context
		eventId string
		id      string
		ch      chan eventResponseData
	}

	eventResponseData string
)

// eventStore

func init() {
	InitEventStore()
}

func InitEventStore() {
	if store != nil {
		return
	}

	store = &eventStore{
		events: make(map[string]*event),
	}
}

func GetEventStore() *eventStore {
	if store == nil {
		store = &eventStore{}
	}
	return store
}

func CloseStore() {
	for id, e := range store.events {
		for _, c := range e.connections {
			close(c)
		}
		close(e.stream)
		delete(store.events, id)
	}
}

// eventResponseData

func constructResponse(data interface{}) eventResponseData {
	b, err := json.Marshal(data)
	if err != nil {
		return eventResponseData("") // no event will be send
	}
	return eventResponseData("id:" + uuid.NewString() + "\n\ndata: " + string(b) + "\n\n")
}

func (erd eventResponseData) empty() bool {
	return erd == ""
}

// event

func NewEvent(id string) *event {
	e := &event{
		id:          id,
		stream:      make(chan eventResponseData),
		connections: make(map[string]chan eventResponseData),
	}

	go func() {
		for data := range e.stream {
			if data.empty() {
				continue
			}

			for _, c := range e.connections {
				c <- data
			}
		}
	}()

	return e
}

func (es *eventStore) CloseEvent(id string) *event {

	e, ok := es.events[id]
	if ok {
		close(e.stream)
		delete(es.events, id)
	}

	return e
}

func (es *eventStore) GetEvent(id string) *event {

	e, ok := es.events[id]
	if !ok {
		e = NewEvent(id)
		es.events[id] = e
	}

	return e
}

func (e *event) GetId() string {
	if e == nil {
		return ""
	}

	return e.id
}

func (e *event) Publish(data interface{}) bool {
	if e == nil || e.stream == nil {
		return false
	}

	e.stream <- constructResponse(data)

	return true
}

// listener

func (e *event) Join(ctx context.Context) *listener {
	if e == nil {
		return nil
	}

	uId := uuid.New().String()
	conn := make(chan eventResponseData)
	e.connections[uId] = conn

	return &listener{
		ctx:     ctx,
		eventId: e.id,
		id:      uId,
		ch:      conn,
	}
}

func (l *listener) Exit() {
	if l == nil {
		return
	}

	close(l.ch)
	delete(store.events[l.eventId].connections, l.id)
}

func (l *listener) GetId() string {
	return l.id
}

func (l *listener) ReadChan() <-chan eventResponseData {
	return l.ch
}

func (l *listener) WaitAndListen(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.(http.Flusher).Flush()

	for {
		select {
		case <-l.ctx.Done():
			log.Println("client disconnected ", l.GetId())
			return

		case e, ok := <-l.ReadChan():
			if !ok {
				log.Println("client channel closed")
				return
			}

			w.Write([]byte(e))
			w.(http.Flusher).Flush()
		}

	}
}
