package main

import (
	"encoding/json"
	"fmt"
)

// WebSocket message types
type MessageType string

const (
	// Client Requests
	ReqJoinQueue    MessageType = "join_queue"
	ReqLeaveQueue   MessageType = "leave_queue"
	ReqEnterGame    MessageType = "enter_game"
	ReqExitGame     MessageType = "exit_game"
	ReqPlayerUpdate MessageType = "player_update"
	// ReqPlayerReady   MessageType = "player_ready"

	// Server Responses
	RespGameState                MessageType = "game_state"
	RespQueueJoined              MessageType = "queue_joined"
	RespQueueLeft                MessageType = "queue_left"
	RespGameConfirmed            MessageType = "game_confirmed"
	RespPlayerEntered            MessageType = "player_entered"
	RespPlayerExited             MessageType = "player_exited"
	RespSecondsToNextRoundStart  MessageType = "secs_round_start"
	RespSecondsToCurrentRoundEnd MessageType = "secs_next_round"
	RespRoundResult              MessageType = "round_result"
)

// Message is the base interface that all messages must implement
type Message interface {
	Type() MessageType
}

// TypedMessage adds type safety to message parsing
type TypedMessage[T Message] struct {
	MessageType MessageType     `json:"messageType"`
	Payload     json.RawMessage `json:"payload"`
}

// CreateMessage creates a typed message from a Message
func CreateMessage[T Message](msg T) (*TypedMessage[T], error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &TypedMessage[T]{
		MessageType: msg.Type(),
		Payload:     payload,
	}, nil
}

// ParseMessage parses a message into its concrete type
func ParseMessage[T Message](data []byte) (*T, error) {
	var base TypedMessage[T]
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	var msg T
	if err := json.Unmarshal(base.Payload, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// Message implementations

// JoinQueueMessage represents a client requesting to join a queue
type JoinQueueMessage struct {
	GameMode GameMode `json:"game_mode"`
	Username string   `json:"name"`
	Flag     string   `json:"flag"`
}

func (m JoinQueueMessage) Type() MessageType {
	return ReqJoinQueue
}

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
	// Creating a message
	joinMsg := JoinQueueMessage{
		GameMode: ModeSprint,
		Username: "Test Player 1",
		Flag:     "🏳️",
	}

	// Create the typed message
	typedMsg, err := CreateMessage(joinMsg)
	if err != nil {
		panic(err)
	}

	// Marshal for transmission
	data, err := json.Marshal(typedMsg)
	if err != nil {
		panic(err)
	}

	// Parse back into concrete type - no type assertion needed!
	parsed, err := ParseMessage[JoinQueueMessage](data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Received data: %s\nMessageType %T\nParsed: %+v\n", data, parsed, parsed)

}
