package main

import (
	"encoding/gob"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	mu sync.RWMutex
	m  *MineField

	persistencePath string

	// trigger channels for updating currently connected players
	updateChannels map[chan bool]bool

	// currently active Players, or Players that have not been gone for too long
	Players map[string]*Player
}

func NewServer(m *MineField, persistencePath string) *Server {
	return &Server{
		m:               m,
		persistencePath: persistencePath,
		updateChannels:  make(map[chan bool]bool),
		Players:         make(map[string]*Player),
	}
}

func (s *Server) Persist() error {
	log.Println("persisting player list")

	s.mu.RLock()
	defer s.mu.RUnlock()

	fh, err := os.Create(s.persistencePath)
	if err != nil {
		return err
	}
	defer fh.Close()

	encoder := gob.NewEncoder(fh)
	err = encoder.Encode(s.Players)
	if err != nil {
		return err
	}

	log.Println("player list persisted")
	return nil
}

// AddPlayer creates a new player for the given ID and returns it. If there is already a player with that ID, it is
// used instead of creating a new player object.
func (s *Server) AddPlayer(id string) *Player {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.Players[id]
	if ok {
		return p
	}

	s.Players[id] = NewPlayer(s, id)
	return s.Players[id]
}

func (s *Server) AddUpdateChannel(ch chan bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updateChannels[ch] = true
}

func (s *Server) RemoveUpdateChannel(ch chan bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.updateChannels, ch)
}

func (s *Server) TriggerGlobalUpdate() {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
