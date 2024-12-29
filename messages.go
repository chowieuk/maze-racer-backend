package main

import (
	"bytes"
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
	Validate() error
	RequiresPayload() bool
}

type BaseMessage struct {
	Type    MessageType     `json:"messageType"`
	Payload json.RawMessage `json:"payload"`
}

// CreateMessage creates a base message from a Message
func CreateMessage[T Message](msg T) (*BaseMessage, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &BaseMessage{
		Type:    msg.Type(),
		Payload: payload,
	}, nil
}

// ParseMessage parses a message into its concrete type
func ParseMessage[T Message](base BaseMessage) (*T, error) {
	var msg T

	// Handle empty payload case
	if len(base.Payload) == 0 || string(base.Payload) == "null" {
		if msg.RequiresPayload() {
			return nil, PayloadRequiredError{MessageType: base.Type}
		}
		return &msg, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(base.Payload))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&msg); err != nil {
		return nil, PayloadFormatError{MessageType: base.Type, Err: err}
	}

	// Validate after decoding
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	return &msg, nil
}

// Message implementations

// JoinQueueMessage represents a client requesting to join a queue
type JoinQueueRequest struct {
	GameMode GameMode `json:"game_mode"`
}

func (m JoinQueueRequest) Type() MessageType {
	return ReqJoinQueue
}

func (m JoinQueueRequest) Validate() error {
	switch m.GameMode {
	case ModeSprint, ModeRace:
		return nil
	default:
		return ValidationError{
			MessageType: ReqJoinQueue,
			Field:       "game_mode",
			Reason:      fmt.Sprintf("must be one of: %v, %v", ModeSprint, ModeRace),
		}
	}

}

func (m JoinQueueRequest) RequiresPayload() bool { return true }

// LeaveQueueMessage represents a client requesting to leave a queue
type LeaveQueueRequest struct{}

func (m LeaveQueueRequest) Type() MessageType {
	return ReqLeaveQueue
}

func (m LeaveQueueRequest) Validate() error {
	return nil
}

func (m LeaveQueueRequest) RequiresPayload() bool { return false }

// LeaveQueueMessage represents a client requesting to leave a queue
type PlayerUpdateRequest struct {
	Level    int      `json:"level"`
	Position Position `json:"position"`
	Rotation float64  `json:"rotation"`
}

func (m PlayerUpdateRequest) Type() MessageType {
	return ReqPlayerUpdate
}

func (m PlayerUpdateRequest) Validate() error {
	if m.Level < 1 {
		return fmt.Errorf("level must be greater than 0")
	}
	// TODO: add position and rotation validation as needed
	return nil
}

func (m PlayerUpdateRequest) RequiresPayload() bool { return true }

// Message-related errors
type ValidationError struct {
	MessageType MessageType
	Field       string
	Reason      string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s %s", e.MessageType, e.Field, e.Reason)
}

type PayloadRequiredError struct {
	MessageType MessageType
}

func (e PayloadRequiredError) Error() string {
	return fmt.Sprintf("payload required for message type %s", e.MessageType)
}

type PayloadFormatError struct {
	MessageType MessageType
	Err         error
}

func (e PayloadFormatError) Error() string {
	return fmt.Sprintf("invalid format for %s message payload: %v", e.MessageType, e.Err)
}
