package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMessage(t *testing.T) {
	testCases := []struct {
		name            string
		input           []byte
		expectedPayload interface{}
		wantErr         bool
		expectedError   string
	}{
		{
			name: "valid join queue request",
			input: []byte(`{
				"messageType": "join_queue",
				"payload": {
					"game_mode": "sprint",
					"username": "Test Player",
					"flag": "🏳️"
				}
			}`),
			expectedPayload: &JoinQueueRequest{
				GameMode: ModeSprint,
				Username: "Test Player",
				Flag:     "🏳️",
			},
			wantErr: false,
		},
		{
			name:            "valid leave queue request",
			input:           []byte(`{"messageType": "leave_queue"}`),
			wantErr:         false,
			expectedPayload: &LeaveQueueRequest{},
		},
		{
			name: "valid player update request",
			input: []byte(`{
				"messageType": "player_update",
				"payload": {
					"level": 1,
					"position":{"x":1.0, "y":2.0},
					"rotation": 45.0
				}
			}`),
			expectedPayload: &PlayerUpdateRequest{
				Level: 1,
				Position: Position{
					X: 1.0,
					Y: 2.0,
				},
				Rotation: 45.0,
			},
			wantErr: false,
		},
		{
			name: "invalid message type",
			input: []byte(`{
				"messageType": "invalid",
				"payload": {}
			}`),
			expectedPayload: nil,
			wantErr:         true,
		},
		{
			name: "invalid player update payload",
			input: []byte(`{
				"messageType": "player_update",
				"payload": {
					"invalid" : "test"
				}
			}`),
			expectedPayload: nil,
			wantErr:         true,
		},
		{
			name: "missing payload",
			input: []byte(`{
				"messageType": "player_update"
			}`),
			expectedPayload: nil,
			wantErr:         true,
		},
		{
			name: "empty payload",
			input: []byte(`{
				"messageType": "player_update", 
				"payload": {}
			}`),
			expectedPayload: nil,
			wantErr:         true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var base BaseMessage
			err := json.Unmarshal(tc.input, &base)
			assert.NoError(t, err, "Failed to unmarshal base message")

			var result interface{}
			var parseErr error
			switch base.Type {
			case ReqJoinQueue:
				result, parseErr = ParseMessage[JoinQueueRequest](base)

			case ReqLeaveQueue:
				result, parseErr = ParseMessage[LeaveQueueRequest](base)

			case ReqPlayerUpdate:
				result, parseErr = ParseMessage[PlayerUpdateRequest](base)

			default:
				parseErr = fmt.Errorf("unknown message type: %s", base.Type)
			}

			if tc.wantErr {
				assert.Error(t, parseErr, "Expected an error but got none")
			} else {
				assert.NoError(t, parseErr, "Unexpected error")
				assert.EqualValues(t, tc.expectedPayload, result)
			}
		})
	}
}
