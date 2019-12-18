package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Println("got request for path", r.URL.Path)

	cookie, err := r.Cookie("sweeperID")
	if err != nil {
		log.Println("Generating ID cookie")
		id := uuid.New().String()
		cookie = &http.Cookie{
			Name:  "sweeperID",
			Value: id,
		}
		http.SetCookie(w, cookie)
	} else {
		log.Println("Got sweeper ID:", cookie.Value)
	}

	fs := http.FileServer(http.Dir("./static"))
	fs.ServeHTTP(w, r)
}

func main() {
	m, err := NewMineField(4, "minefield.gob")
	if err != nil {
		log.Fatalln("can't create mine field:", err)
	}

	log.Println("Registering HTTP handlers")

	s, err := NewServer(m, "server.gob")
	if err != nil {
		log.Fatalln("can't create server:", err)
	}
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", s.wsHandler)

	log.Println("HTTP handler set up, listening on port 8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln("can't run http server:", err)
	}
}
