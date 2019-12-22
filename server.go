package main

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"

	"github.com/google/uuid"
)

const _numHighscores = 20
const _anonName = "Etaoin Shrdlu"

// HighscoreEntry is an entry in the highscores table
type HighscoreEntry struct {
	Name  string
	Score uint
}

type Server struct {
	mu sync.RWMutex
	m  *MineField

	persistencePath string

	// trigger channels for updating currently connected players
	updateChannels map[chan bool]bool

	// currently active Players, or Players that have not been gone for too long
	Players map[string]*Player
}

func NewServer(m *MineField, persistencePath string) (*Server, error) {
	s := &Server{
		m:               m,
		persistencePath: persistencePath,
		updateChannels:  make(map[chan bool]bool),
		Players:         make(map[string]*Player),
	}

	fh, err := os.Open(persistencePath)
	if err != nil {
		log.Println("can't load server state, using fresh server:", err)
		return s, nil
	}
	defer fh.Close()

	dec := gob.NewDecoder(fh)
	err = dec.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("can't load server: %w", err)
	}

	// Initialize dynamic components of the server
	for _, p := range s.Players {
		p.setServer(s)
	}

	log.Println("loaded server state, players:", s.Players)

	return s, nil
}

func (s *Server) GetHighscores() []HighscoreEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scores := make([]HighscoreEntry, 0)
	for _, p := range s.Players {
		p.mu.RLock()
		entry := HighscoreEntry{
			Name:  _anonName,
			Score: p.Score,
		}
		if p.Name != "" {
			entry.Name = p.Name
		}
		p.mu.RUnlock()
		scores = append(scores, entry)
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Name < scores[j].Name
		}
		return scores[i].Score > scores[j].Score
	})

	if len(scores) > _numHighscores {
		scores = scores[:_numHighscores]
	}
	return scores
}

func (s *Server) Persist() error {
	log.Println("persisting player list")

	s.mu.RLock()
	defer s.mu.RUnlock()

	fh, err := ioutil.TempFile(".", s.persistencePath)
	if err != nil {
		return err
	}
	defer fh.Close()

	encoder := gob.NewEncoder(fh)
	err = encoder.Encode(s)
	if err != nil {
		return err
	}

	err = os.Rename(fh.Name(), s.persistencePath)

	log.Println("player list persisted")
	return err
}

// AddPlayer creates a new player for the given ID and returns it. If there is already a player with that ID, it is
// used instead of creating a new player object.
func (s *Server) AddPlayer(id string) *Player {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.Players[id]
	if ok {
		log.Println("using stored player", p)
		return p
	}

	log.Println("generating new player")

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
		r.Body.Close()
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
