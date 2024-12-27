package main

// GameMode represents the available game modes
type GameMode string

const (
	ModeSprint GameMode = "sprint"
	ModeRace   GameMode = "race"
)

// Player represents a specific player entity in a game
type Player struct {
	Id       string   `json:"id"`
	Active   bool     `json:"active"`
	Name     string   `json:"name"`
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

func main() {
	// Main application logic will go here
}
