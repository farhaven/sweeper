package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

const _viewPortWidth = 20
const _viewPortHeight = 20

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ClientRequest struct {
	Kind string // kind of request: 'move', 'uncover', 'mark'
	X, Y int    // parameters: deltaX, deltaY for move, X and Y relative to viewport for click
}

type Server struct {
	sync.RWMutex
	m              *MineField
	updateChannels map[chan bool]bool
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
	log.Println("Got new websocket request")

	// - upgrade websocket
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Can't upgrade websocket connection: %s", err)
		return
	}
	defer conn.Close()

	// - send initial viewport
	viewport := image.Rect(-_viewPortWidth/2, -_viewPortHeight/2, _viewPortWidth/2, _viewPortHeight/2)
	updateViewport := make(chan bool)
	s.AddUpdateChannel(updateViewport)
	defer s.RemoveUpdateChannel(updateViewport)
	defer close(updateViewport)
	go func() {
		for range updateViewport {
			wr, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("can't get writer for websocket:", err)
				return
			}
			enc := json.NewEncoder(wr)
			err = enc.Encode(s.m.ExtractPlayerView(viewport))
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
			viewport.Min.X += req.X
			viewport.Max.X += req.X
			viewport.Min.Y += req.Y
			viewport.Max.Y += req.Y
			// Trigger local viewport update
			select {
			case updateViewport <- true:
			default:
			}
		case "uncover":
			s.m.Uncover(viewport.Min.X+req.X, viewport.Min.Y+req.Y)
			// TODO: Only trigger updates in overlapping viewports
			s.TriggerGlobalUpdate()
		case "mark":
			log.Println("mark request", req)
			// TODO: Only trigger updates in overlapping viewports
			s.m.Mark(viewport.Min.X+req.X, viewport.Min.Y+req.Y)
			s.TriggerGlobalUpdate()
		default:
			log.Printf("invalid request: %#v", req)
			return
		}
	}
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
	m, err := NewMineField(4)
	if err != nil {
		log.Fatalln("can't create mine field:", err)
	}

	rect := image.Rect(-10, -10, 10, 10)
	field := m.ExtractPlayerView(rect)

	for _, row := range field.Data {
		log.Println(row)
	}

	s := Server{
		m:              m,
		updateChannels: make(map[chan bool]bool),
	}

	log.Println("Registering HTTP handlers")

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", s.wsHandler)

	log.Println("HTTP handler set up")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln("can't run http server:", err)
	}
}
