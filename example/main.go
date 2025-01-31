package main

import (
	"net/http"
	"strings"

	"github.com/VaishnavcveeZ/go-sse-event/sseevent"

	"github.com/gorilla/mux"
)

var es *sseevent.EventStore

func main() {

	es = sseevent.NewEventStoreInstance()
	defer es.CloseStore()

	r := mux.NewRouter()

	r.HandleFunc("/sse/event/{id}/listen", EventListener).Methods("GET")
	r.HandleFunc("/sse/event/{id}/publish/{data}", EventDataPublish).Methods("GET")

	// event publish with user control
	r.HandleFunc("/sse/event/{id}/user/{user}/listen", EventUserListener).Methods("GET")
	// ?users=1,2
	r.HandleFunc("/sse/event/{id}/new-publish/{data}", EventUserDataPublish).Methods("GET")

	http.ListenAndServe(":8800", r)
}

func EventDataPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	data := vars["data"]

	event := es.GetEvent(id)

	event.Publish(map[string]any{
		"eventId": id,
		"info":    data,
	})

}

func EventListener(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	event := es.GetEvent(id)

	conn := event.Join(r.Context())
	defer conn.Exit()

	conn.WaitAndListen(w)
}

// user controled event publish

func EventUserDataPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	data := vars["data"]
	users := strings.Split(r.URL.Query().Get("users"), ",")

	event := es.GetEvent(id)

	publisher := event.NewPublisher()

	publisher.SetListeners(users...)
	// publisher.SetListenersExcept(users...)

	publisher.PublishData(map[string]any{
		"eventId": id,
		"info":    data,
	})

}

func EventUserListener(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	user := vars["user"]

	event := es.GetEvent(id)

	conn := event.Join(r.Context()).SetListenerId(user)
	defer conn.Exit()

	conn.WaitAndListen(w)
}
