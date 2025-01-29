package main

import (
	"net/http"

	"github.com/VaishnavcveeZ/go-sse-event-stream/sseevent"

	"github.com/gorilla/mux"
)

func main() {

	sseevent.InitEventStore()
	defer sseevent.CloseStore()

	r := mux.NewRouter()

	r.HandleFunc("/sse/event/{id}/listen", EventListener).Methods("GET")
	r.HandleFunc("/sse/event/{id}/publish/{data}", EventDataPublish).Methods("GET")

	http.ListenAndServe(":8800", r)
}

func EventDataPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	data := vars["data"]

	event := sseevent.GetEventStore().GetEvent(id)

	event.Publish(map[string]any{
		"eventId": id,
		"info":    data,
	})

}

func EventListener(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	event := sseevent.GetEventStore().GetEvent(id)

	conn := event.Join(r.Context())
	defer conn.Exit()

	conn.WaitAndListen(w)
}
