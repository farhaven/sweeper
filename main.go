package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"

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

type Server struct {
	sync.RWMutex
	m *MineField

	// trigger channels for updating currently connected players
	updateChannels map[chan bool]bool

	// currently active players, or players that have not been gone for too long
	players map[string]*Player
}

func NewServer(m *MineField) *Server {
	return &Server{
		m:              m,
		updateChannels: make(map[chan bool]bool),
		players:        make(map[string]*Player),
	}
}

// AddPlayer creates a new player for the given ID and returns it. If there is already a player with that ID, it is
// used instead of creating a new player object.
func (s *Server) AddPlayer(id string) *Player {
	s.Lock()
	defer s.Unlock()

	p, ok := s.players[id]
	if ok {
		return p
	}

	s.players[id] = NewPlayer(s, id)
	return s.players[id]
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

	var playerID string
	idCookie, err := r.Cookie("sweeperID")
	if err != nil {
		log.Println("No sweeper ID cookie supplied, using a new randomly generated ID:", err)
		playerID = uuid.New().String()
	} else {
		playerID = idCookie.Value
	}

	p := s.AddPlayer(playerID)
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
