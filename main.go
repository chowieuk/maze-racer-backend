package main

import (
	"encoding/json"
	"fmt"
)

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

	// Example raw messages
	rawMessages := [][]byte{
		[]byte(`{
			"messageType": "join_queue",
			"payload": {
				"game_mode": "sprint",
				"username": "Test Player",
				"flag": "🏳️"
			}
		}`),
		[]byte(`{"messageType": "leave_queue"}`),
		[]byte(`{
			"messageType": "player_update",
			"payload": {
				"level": 1,
				"position":{"x":1.0, "y":2.0},
				"rotation": 45.0
			}
		}`),
		[]byte(`{
			"messageType": "invalid",
			"payload": {}
		}`),
		[]byte(`{"messageType": "player_update","payload": {"invalid" : "test"}}`),
		[]byte(`{
			"messageType": "player_update",
		}`),
		[]byte(`{
			"messageType": "player_update", "payload": {}
		}`),
	}

	for _, rawMsg := range rawMessages {

		var msg BaseMessage
		err := json.Unmarshal(rawMsg, &msg)
		if err != nil {
			fmt.Printf("error parsing into BaseMessage: %v\n", err)
			continue
		}

		switch msg.Type {
		case ReqJoinQueue:
			parsed, err := ParseMessage[JoinQueueRequest](msg)
			if err != nil {
				fmt.Printf("parse err: %s\n", err)
			}
			fmt.Printf("Message Type: %T, payload: %+v\n", parsed, parsed)
		case ReqLeaveQueue:
			parsed, err := ParseMessage[LeaveQueueRequest](msg)
			if err != nil {
				fmt.Printf("parse err: %s\n", err)
			}
			fmt.Printf("Message Type: %T, payload: %+v\n", parsed, parsed)
		case ReqPlayerUpdate:
			parsed, err := ParseMessage[PlayerUpdateRequest](msg)
			if err != nil {
				fmt.Printf("error parsing %s:\n%s\n", rawMsg, err)
				break
			}
			fmt.Printf("Message Type: %T, payload: %+v\n", parsed, parsed)
		default:
			fmt.Printf("Unknown Message Type / Format: %s\n", rawMsg)

		}
	}

}
