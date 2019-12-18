package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

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
