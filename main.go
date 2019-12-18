package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

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

	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "can't find resource for path %s", r.URL.Path)
		return
	}

	fh, err := os.Open("index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "can't open index: %s", err)
		return
	}
	defer fh.Close()

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
	w.Header().Add("content-type", "text/html")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, fh)
}

func main() {
	m, err := NewMineField(4, "minefield.gob")
	if err != nil {
		log.Fatalln("can't create mine field:", err)
	}

	log.Println("Registering HTTP handlers")

	s := NewServer(m)
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", s.wsHandler)

	log.Println("HTTP handler set up, listening on port 8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln("can't run http server:", err)
	}
}
