package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

var eventStore *EventStore

type Event struct {
	id          string
	stream      chan EventData
	connections map[string]chan string
}

type EventData struct {
	SenderId string
	Data     string
}

func (ed *EventData) Construct() string {
	return fmt.Sprintf("Sender:%s;Data:%s;Time:%d;\n", ed.SenderId, ed.Data, time.Now().Unix())
}

func NewEvent(id string) *Event {
	e := &Event{
		id:          id,
		stream:      make(chan EventData),
		connections: make(map[string]chan string),
	}

	go func() {
		for ed := range e.stream {
			content := ed.Construct()
			for _, c := range e.connections {
				c <- content
			}
		}
	}()

	return e
}

func (e *Event) GetId() string {
	if e == nil {
		return ""
	}

	return e.id
}

func (e *Event) GetStream() chan EventData {
	if e == nil {
		return nil
	}

	return e.stream
}

func (e *Event) Join() (string, chan string) {
	if e == nil {
		return "", nil
	}

	uId := uuid.New().String()
	conn := make(chan string)
	e.connections[uId] = conn
	return uId, conn
}

func (e *Event) Exit(uId string) {
	if e == nil {
		return
	}

	conn := e.connections[uId]
	close(conn)
	delete(e.connections, uId)
}

type EventStore struct {
	events map[string]*Event
}

func InitEventStore() {
	if eventStore != nil {
		return
	}

	eventStore = &EventStore{
		events: make(map[string]*Event),
	}
}

func ShutDownStore() {
	for id, e := range eventStore.events {
		for _, c := range e.connections {
			close(c)
		}
		close(e.stream)
		delete(eventStore.events, id)
	}
}

func (es *EventStore) GetEvent(id string) *Event {
	InitEventStore()

	e, ok := es.events[id]
	if !ok {
		e = NewEvent(id)
		es.events[id] = e
	}

	return e
}

func (es *EventStore) JoinEvent(id string) *Event {
	InitEventStore()

	e, ok := es.events[id]
	if !ok {
		e = NewEvent(id)
		es.events[id] = e
	}

	return e
}

func (es *EventStore) CloseEvent(id string) *Event {
	InitEventStore()

	e, ok := es.events[id]
	if ok {
		close(e.stream)
		delete(es.events, id)
	}

	return e
}
