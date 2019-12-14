package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

const _viewPortWidth = 10
const _viewPortHeight = 10

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ClientRequest struct {
	Kind string // kind of request: 'move' or 'click'
	X, Y int    // parameters: deltaX, deltaY for move, X and Y relative to viewport for click
}

type Server struct {
	m *MineField
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
	// - register websocket in server, defer removal
	// - send events to user:
	//   - field changed (because someone clicked on it, or the viewpoint was moved)
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

		// TODO:
		// - handle user requests:
		//   - move viewport
		//   - click on field
		switch req.Kind {
		case "move":
			viewport.Min.X += req.X
			viewport.Max.X += req.X
			viewport.Min.Y += req.Y
			viewport.Max.Y += req.Y
		case "click":
			s.m.HandleClick(viewport.Min.X + req.X, viewport.Min.Y + req.Y)
			// Trigger viewport update
		default:
			log.Printf("invalid request: %#v", req)
			return
		}

		// Trigger viewport update
		select {
		case updateViewport <- true:
		default:
		}
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Println("got request for index")

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

/*
- Render field
- Build flood fill, up to a certain maximum radius
*/
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

	// os.Exit(0)

	s := Server{
		m: m,
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
