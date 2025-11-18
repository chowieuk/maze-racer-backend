package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPlayer(t *testing.T) {
	username := "testUser"
	flag := "🏴"

	player := NewPlayer(username, flag)

	assert.Equal(t, username, player.Username)
	assert.Equal(t, flag, player.Flag)
	assert.False(t, player.Active)
	assert.Equal(t, 1, player.Level)
	assert.Equal(t, -1000.0, player.Position.X)
	assert.Equal(t, -1000.0, player.Position.Y)
	assert.Equal(t, 0.0, player.Rotation)
	assert.NotEmpty(t, player.Id)
}

func TestGameStateScoring(t *testing.T) {
	testsCases := []struct {
		name     string
		setup    func(*GameState)
		expected string
		wantErr  bool
	}{
		{
			name:     "empty game state",
			setup:    func(gs *GameState) {},
			expected: `{"playerScores":[]}`,
			wantErr:  false,
		},
		{
			name: "single player",
			setup: func(gs *GameState) {
				player := NewPlayer("player1", "US")
				player.Level = 5
				gs.Players.Set(player.Id, player)
			},
			expected: `{"playerScores":[{"username":"player1","flag":"US","level":5}]}`,
			wantErr:  false,
		},
		{
			name: "multiple players sorted by level",
			setup: func(gs *GameState) {
				p1 := NewPlayer("player1", "US")
				p1.Level = 5
				gs.Players.Set(p1.Id, p1)

				p2 := NewPlayer("player2", "UK")
				p2.Level = 3
				gs.Players.Set(p2.Id, p2)

				p3 := NewPlayer("player3", "FR")
				p3.Level = 7
				gs.Players.Set(p3.Id, p3)
			},
			expected: `{"playerScores":[{"username":"player3","flag":"FR","level":7},{"username":"player1","flag":"US","level":5},{"username":"player2","flag":"UK","level":3}]}`,
			wantErr:  false,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewGameState(123) // seed value doesn't matter for these tests
			tc.setup(gs)

			round := gs.GetRoundResult()
			bytes, err := json.Marshal(round)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.JSONEq(t, tc.expected, string(bytes))
			}
		})
	}
}
