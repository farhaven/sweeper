package main

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type ClientRequest struct {
	Kind string // kind of request: 'move', 'uncover', 'mark'
	X, Y int    // parameters: deltaX, deltaY for move, X and Y relative to viewport for click
}

const _viewPortWidth = 20
const _viewPortHeight = 20

type Player struct {
	mu       sync.RWMutex
	s        *Server
	Viewport image.Rectangle
	Score    uint
	Id       string
}

func NewPlayer(s *Server, id string) *Player {
	log.Println("Player with ID", id, "connected")
	return &Player{
		s:        s,
		Viewport: image.Rect(-_viewPortWidth/2, -_viewPortHeight/2, _viewPortWidth/2, _viewPortHeight/2),
		Id:       id,
	}
}

func (p *Player) String() string {
	return fmt.Sprintf("%s@%s/%d", p.Id, p.Viewport, p.Score)
}

func (p *Player) setServer(s *Server) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.s = s
}

func (p *Player) mapViewport(req ClientRequest) (int, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Viewport.Min.X + req.X, p.Viewport.Min.Y + req.Y
}

func (p *Player) shiftViewport(deltaX int, deltaY int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Viewport.Min.X += deltaX
	p.Viewport.Max.X += deltaX
	p.Viewport.Min.Y += deltaY
	p.Viewport.Max.Y += deltaY
}

func (p *Player) getScore() uint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Score
}

func (p *Player) incScore(delta uint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Score += delta
}

func (p *Player) resetScore() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Score = 0
}

// A state update contains the current score and the rendered viewpoint of a player, as well as the current high score list
type StateUpdate struct {
	Score      uint
	ViewPort   ViewPort
	Highscores []HighscoreEntry
}

func (p *Player) Loop(conn *websocket.Conn) {
	updateViewport := make(chan bool)
	p.s.AddUpdateChannel(updateViewport)
	defer p.s.RemoveUpdateChannel(updateViewport)
	defer close(updateViewport)
	go func() {
		// Rate limiter for updates
		limit := rate.NewLimiter(3, 5)

		for range updateViewport {
			if !limit.Allow() {
				log.Println("Not sending update, rate limit exceeded")
			}
			wr, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("can't get writer for websocket:", err)
				return
			}
			enc := json.NewEncoder(wr)
			update := StateUpdate{
				Score:      p.getScore(),
				ViewPort:   p.s.m.ExtractPlayerView(p.Viewport),
				Highscores: p.s.GetHighscores(),
			}
			err = enc.Encode(update)
			if err != nil {
				log.Println("Can't encode update:", err)
				wr.Close()
				return
			}
			wr.Close()
		}
	}()
	// immediately trigger update
	updateViewport <- true
	// Periodically send updates
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			select {
			case updateViewport <- true:
			default:
			}
		}
	}()
	defer ticker.Stop()

	// TODO:
	// - send events to user:
	//   - game over (someone clicked on a mine)

	for {
		messageType, r, err := conn.NextReader()
		if err != nil {
			return
		}
		log.Println("got message of type", messageType)
		var req ClientRequest
		dec := json.NewDecoder(r)
		err = dec.Decode(&req)
		if err != nil {
			log.Printf("Can't decode client request: %s", err)
		}
		log.Printf("got client request %#v", req)

		// - handle user requests:
		//   - move viewport
		//   - click on field
		switch req.Kind {
		case "move":
			p.shiftViewport(req.X, req.Y)
			// Trigger local viewport update
			select {
			case updateViewport <- true:
			default:
			}
			err = p.s.Persist()
			if err != nil {
				log.Println("can't persist player list:", err)
			}
		case "uncover":
			result, uncovered := p.s.m.Uncover(p.mapViewport(req))
			if result != UncoverBoom {
				p.incScore(uncovered)
			} else {
				// TODO: Notify player with a "BOOM" message or something
				p.resetScore()
			}
			err = p.s.m.Persist()
			if err != nil {
				log.Println("can't persist minefield:", err)
			}
			err = p.s.Persist()
			if err != nil {
				log.Println("can't persist player list:", err)
			}
			// TODO: Only trigger updates in overlapping viewports
			p.s.TriggerGlobalUpdate()
		case "mark":
			log.Println("mark request", req)
			// TODO: Only trigger updates in overlapping viewports
			p.s.m.Mark(p.mapViewport(req))
			err = p.s.m.Persist()
			if err != nil {
				log.Println("can't persist minefield:", err)
			}
			p.s.TriggerGlobalUpdate()
		default:
			log.Printf("invalid request: %#v", req)
			return
		}
	}
}
