package sseevent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
)

var (
	store         *eventStore
	listenersPool *listenerConnectionsPool
)

type (
	eventStore struct {
		events map[string]*event
	}

	event struct {
		id          string
		stream      chan eventPublishData
		connections map[string]*listener
	}

	publisher struct {
		event             *event
		err               error
		listenerIds       map[string]bool
		exceptListenerIds map[string]bool
	}

	eventPublishData struct {
		data              eventResponseData
		listenerIds       map[string]bool
		exceptListenerIds map[string]bool
	}

	listener struct {
		ctx            context.Context
		eventId        string
		connectionId   string
		connectionChan chan eventResponseData
		listenerId     string
	}

	listenerConnectionsPool struct {
		listenerConnIds map[string]map[string]bool
	}

	eventResponseData string
)

// eventStore

func init() {
	InitEventStore()
}

func InitEventStore() {
	if store == nil {
		store = &eventStore{
			events: make(map[string]*event),
		}
	}

	if listenersPool == nil {
		listenersPool = &listenerConnectionsPool{
			listenerConnIds: make(map[string]map[string]bool),
		}
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
			close(c.connectionChan)
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
		stream:      make(chan eventPublishData),
		connections: make(map[string]*listener),
	}

	go func() {
		for eventData := range e.stream {
			if eventData.data.empty() {
				continue
			}

			for _, c := range e.connections {
				if eventData.exceptListenerIds != nil {
					if ok := eventData.exceptListenerIds[c.listenerId]; ok {
						continue
					}
				}

				if eventData.listenerIds != nil && len(eventData.listenerIds) > 0 {
					if ok := eventData.listenerIds[c.listenerId]; !ok || c.listenerId == "" {
						continue
					}
				}

				c.connectionChan <- eventData.data
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

	e.stream <- eventPublishData{
		data: constructResponse(data),
	}

	return true
}

// publisher

func (e *event) NewPublisher() *publisher {
	if e == nil || e.stream == nil {
		return nil
	}

	return &publisher{
		event:             e,
		listenerIds:       make(map[string]bool),
		exceptListenerIds: make(map[string]bool),
	}
}

func (p *publisher) SetListeners(listenerIds ...string) *publisher {

	for _, l := range listenerIds {
		p.listenerIds[l] = true
	}
	return p
}

func (p *publisher) ExceptListeners(listenerIds ...string) *publisher {
	for _, l := range listenerIds {
		p.exceptListenerIds[l] = true
	}
	return p
}

func (p *publisher) PublishData(data interface{}) *publisher {

	p.event.stream <- eventPublishData{
		data:              constructResponse(data),
		listenerIds:       p.listenerIds,
		exceptListenerIds: p.exceptListenerIds,
	}

	return p
}

func (p *publisher) Error() error {
	return p.err
}

// listenerPool
func addListenerToPool(connectionsId, listenerId string) {
	if _, ok := listenersPool.listenerConnIds[listenerId]; !ok {
		listenersPool.listenerConnIds[listenerId] = make(map[string]bool)
	}

	listenersPool.listenerConnIds[listenerId][connectionsId] = true
}

func removeFromListenerPool(connectionsId, listenerId string) {
	if liConnIds, ok := listenersPool.listenerConnIds[listenerId]; ok {
		delete(liConnIds, connectionsId)
	}
}

// listener

func (e *event) Join(ctx context.Context) *listener {
	if e == nil {
		return nil
	}

	l := &listener{
		ctx:            ctx,
		eventId:        e.id,
		connectionId:   uuid.New().String(),
		connectionChan: make(chan eventResponseData),
	}

	e.connections[l.GetId()] = l

	return l
}

func (l *listener) Exit() {
	if l == nil {
		return
	}

	if l.listenerId != "" {
		removeFromListenerPool(l.connectionId, l.listenerId)
	}
	close(l.connectionChan)
	delete(store.events[l.eventId].connections, l.connectionId)
}

func (l *listener) SetListenerId(listenerId string) *listener {
	addListenerToPool(l.connectionId, listenerId)
	l.listenerId = listenerId

	return l
}

func (l *listener) GetId() string {
	return l.connectionId
}

func (l *listener) ReadChan() <-chan eventResponseData {
	return l.connectionChan
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
