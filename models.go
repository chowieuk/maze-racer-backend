package main

import "github.com/google/uuid"

// Player represents a specific player entity in a game
type Player struct {
	Id       string   `json:"id"`
	Active   bool     `json:"active"`
	Username string   `json:"username"`
	Flag     string   `json:"flag"`
	Level    int      `json:"level"`
	Position Position `json:"position"`
	Rotation float64  `json:"rotation"`
}

// Position represents the position of the sprite for a player
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Creates a new player
func NewPlayer(username, flag string) Player {
	return Player{
		Id:       uuid.New().String(),
		Active:   false,
		Username: username,
		Flag:     flag,
		Level:    1,
		Position: Position{
			X: -1000,
			Y: -1000,
		},
		Rotation: 0,
	}
}

// Creates a new player from a request
func NewPlayerFromRequest(req JoinQueueRequest) Player {
	return NewPlayer(req.Username, req.Flag)
}
