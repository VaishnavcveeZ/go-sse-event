package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func main() {

	InitEventStore()
	defer ShutDownStore()

	r := mux.NewRouter()

	r.HandleFunc("/listen/event/{id}", EventListener).Methods("GET")
	r.HandleFunc("/publish/event/{id}", EventPublish).Methods("GET")
	r.HandleFunc("/publish/event/{id}/data/{data}", EventDataPublish).Methods("GET")

	http.ListenAndServe(":8800", r)
}

func EventPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	event := eventStore.GetEvent(id)

	event.GetStream() <- EventData{
		SenderId: "xxx",
		Data:     "id: " + id + " START\n",
	}
	for i := 0; i < 10; i++ {

		select {
		case <-r.Context().Done():
			return

		default:
			event.GetStream() <- EventData{
				SenderId: "xxx",
				Data:     fmt.Sprint(i) + " id: " + id + " - " + RandStringRunes(15),
			}
			time.Sleep(time.Millisecond * 3000)
		}
	}
	event.GetStream() <- EventData{
		SenderId: "xxx",
		Data:     "\nEND",
	}
}

func EventDataPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	data := vars["data"]

	event := eventStore.GetEvent(id)

	event.GetStream() <- EventData{
		SenderId: "xxx",
		Data:     data,
	}

}

func EventListener(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	event := eventStore.GetEvent(id)

	uid, conn := event.Join()
	defer event.Exit(uid)

	w.Header().Set("Content-Type", "text/event-stream")

	w.Write([]byte("Listening... " + id + "\n\n"))
	w.(http.Flusher).Flush()

	for {
		select {
		case <-r.Context().Done():
			fmt.Println("req closed ", uid)
			return

		case e, open := <-conn:
			if !open {
				fmt.Println("chan closed")
				return
			}

			w.Write([]byte(e + "\n"))
			w.(http.Flusher).Flush()
		}

	}
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
