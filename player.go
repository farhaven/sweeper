package main

import (
	"encoding/json"
	"image"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type ClientRequest struct {
	Kind string // kind of request: 'move', 'uncover', 'mark'
	X, Y int    // parameters: deltaX, deltaY for move, X and Y relative to viewport for click
}

const _viewPortWidth = 20
const _viewPortHeight = 20

type Player struct {
	sync.RWMutex
	s        *Server
	viewport image.Rectangle
}

func NewPlayer(s *Server) *Player {
	return &Player{
		s:        s,
		viewport: image.Rect(-_viewPortWidth/2, -_viewPortHeight/2, _viewPortWidth/2, _viewPortHeight/2),
	}
}


func (p *Player) shiftViewport(deltaX int, deltaY int) {
	p.Lock()
	defer p.Unlock()

	p.viewport.Min.X += deltaX
	p.viewport.Max.X += deltaX
	p.viewport.Min.Y += deltaY
	p.viewport.Max.Y += deltaY
}

func (p *Player) Loop(conn *websocket.Conn) {
	updateViewport := make(chan bool)
	p.s.AddUpdateChannel(updateViewport)
	defer p.s.RemoveUpdateChannel(updateViewport)
	defer close(updateViewport)
	go func() {
		for range updateViewport {
			wr, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("can't get writer for websocket:", err)
				return
			}
			enc := json.NewEncoder(wr)
			err = enc.Encode(p.s.m.ExtractPlayerView(p.viewport))
			if err != nil {
				log.Println("Can't encode field:", err)
				wr.Close()
				return
			}
			wr.Close()
		}
	}()
	// immediately trigger update
	updateViewport <- true

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
		case "uncover":
			p.s.m.Uncover(p.viewport.Min.X+req.X, p.viewport.Min.Y+req.Y)
			// TODO: Only trigger updates in overlapping viewports
			p.s.TriggerGlobalUpdate()
		case "mark":
			log.Println("mark request", req)
			// TODO: Only trigger updates in overlapping viewports
			p.s.m.Mark(p.viewport.Min.X+req.X, p.viewport.Min.Y+req.Y)
			p.s.TriggerGlobalUpdate()
		default:
			log.Printf("invalid request: %#v", req)
			return
		}
	}
}
