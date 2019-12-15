package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	sync.RWMutex
	m              *MineField
	updateChannels map[chan bool]bool
	players        map[*Player]bool
}

func NewServer(m *MineField) *Server {
	return &Server{
		m:              m,
		updateChannels: make(map[chan bool]bool),
		players:        make(map[*Player]bool),
	}
}

func (s *Server) AddPlayer(p *Player) {
	s.Lock()
	defer s.Unlock()

	s.players[p] = true
}

func (s *Server) RemovePlayer(p *Player) {
	s.Lock()
	defer s.Unlock()

	delete(s.players, p)
}

func (s *Server) AddUpdateChannel(ch chan bool) {
	s.Lock()
	defer s.Unlock()

	s.updateChannels[ch] = true
}

func (s *Server) RemoveUpdateChannel(ch chan bool) {
	s.Lock()
	defer s.Unlock()

	delete(s.updateChannels, ch)
}

func (s *Server) TriggerGlobalUpdate() {
	s.RLock()
	defer s.RUnlock()

	for ch := range s.updateChannels {
		select {
		case ch <- true:
		default:
		}
	}
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	// - upgrade websocket
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Can't upgrade websocket connection: %s", err)
		return
	}
	defer conn.Close()

	p := NewPlayer(s)
	s.AddPlayer(p)
	defer s.RemovePlayer(p)

	log.Println("running loop for player", p)
	p.Loop(conn)
	log.Println("player", p, "disconnected")
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

	w.Header().Add("content-type", "text/html")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, fh)
}

func main() {
	m, err := NewMineField(4, "minefield.json")
	if err != nil {
		log.Fatalln("can't create mine field:", err)
	}

	log.Println("Registering HTTP handlers")

	s := NewServer(m)
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", s.wsHandler)

	log.Println("HTTP handler set up")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln("can't run http server:", err)
	}
}
