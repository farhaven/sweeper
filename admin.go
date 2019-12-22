package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Admins struct {
	Cookies []string
}

func NewAdminsFromFile(path string) (Admins, error) {
	var admins Admins

	fh, err := os.Open("admins.json")
	if err != nil {
		log.Println("Can't get admins file:", err)
		return admins, err
	}
	defer fh.Close()

	dec := json.NewDecoder(fh)
	err = dec.Decode(&admins)

	return admins, err
}

func (a Admins) Allowed(cookie string) bool {
	for _, c := range a.Cookies {
		if c == cookie {
			return true
		}
	}
	return false
}

func isAdminUser(cookie string) bool {
	log.Println("checking admin state for", cookie)

	admins, err := NewAdminsFromFile("admins.json")
	if err != nil {
		log.Println("can't load admins:", err)
		return false
	}

	allowed := admins.Allowed(cookie)

	if allowed {
		log.Println("admin check for", cookie, "passed")
	} else {
		log.Println("admin check denied for", cookie)
	}

	return allowed
}

type AdminRequest struct {
	Request string
}

type PlayerListEntry struct {
	Name  string
	Score uint
}

func (s *Server) adminGetPlayers() []PlayerListEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	players := make([]PlayerListEntry, 0)
	for _, p := range s.Players {
		p.mu.RLock()
		players = append(players, PlayerListEntry{
			Name:  p.Name,
			Score: p.Score,
		})
		p.mu.RUnlock()
	}

	return players
}

func (s *Server) adminGetAdmins() []PlayerListEntry {
	res := make([]PlayerListEntry, 0)

	admins, err := NewAdminsFromFile("admins.json")
	if err != nil {
		log.Println("can't load admins:", err)
		return res
	}

	for c, p := range s.Players {
		if !admins.Allowed(c) {
			continue
		}
		p.mu.RLock()
		res = append(res, PlayerListEntry{
			Name:  p.Name,
			Score: p.Score,
		})
		p.mu.RUnlock()
	}

	return res
}

func (s *Server) adminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Admin handler called for", r)
	defer r.Body.Close()

	var req AdminRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Can't decode request: %s", err)
		log.Println("can't decode admin request:", err)
		return
	}

	log.Println("got admin request:", req)
	switch req.Request {
	case "get-players":
		log.Println("handling get players")
		enc := json.NewEncoder(w)
		players := s.adminGetPlayers()
		enc.Encode(players)
	case "get-admins":
		log.Println("get admins")
		enc := json.NewEncoder(w)
		admins := s.adminGetAdmins()
		enc.Encode(admins)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unknown request: %s", req.Request)
		log.Println("unknown request:", req.Request)
		return
	}
}
