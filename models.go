package main

import (
	"cmp"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
)

// GameState represents the state of a specific game
type GameState struct {
	Id      string                `json:"id"`
	Seed    int64                 `json:"seed"`
	Players CMap[string, *Player] `json:"players"`
}

// NewGameState initializes a thread-safe game instance with the given random seed.
// The returned state includes a unique identifier and a concurrent-safe player registry.
func NewGameState(seed int64) *GameState {
	return &GameState{
		Id:      uuid.New().String(),
		Seed:    seed,
		Players: NewMutexMap[string, *Player](),
	}
}

// AsUpdateMessage Marshalls the current gamestate as JSON bytes
func (gs *GameState) AsUpdateMessage() ([]byte, error) {
	return json.Marshal(struct {
		Type    MessageType `json:"messageType"`
		Payload interface{} `json:"payload"`
	}{
		Type:    RespGameState,
		Payload: gs,
	})
}

// GetRoundResult returns the end-of-round results containing player scores.
// It collects scores from all players in the game state and sorts them
// by level in descending order (highest level first).
func (gs *GameState) GetRoundResult() RoundResult {
	playerScores := make([]PlayerScore, 0, len(gs.Players.Values()))
	for _, p := range gs.Players.Values() {
		score := PlayerScore{
			Username: p.Username,
			Flag:     p.Flag,
			Level:    p.Level,
		}
		playerScores = append(playerScores, score)
	}
	slices.SortFunc(playerScores,
		func(a, b PlayerScore) int {
			return cmp.Compare(b.Level, a.Level)
		})

	return RoundResult{
		PlayerScores: playerScores,
	}
}

// RoundResult represents the end of round results
type RoundResult struct {
	PlayerScores []PlayerScore `json:"playerScores"`
}

// PlayerScore represents an individual players end of round score
type PlayerScore struct {
	Username string `json:"username"`
	Flag     string `json:"flag"`
	Level    int    `json:"level"`
}

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
func NewPlayer(username, flag string) *Player {
	return &Player{
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
