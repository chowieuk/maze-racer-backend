package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"math/rand"

	"github.com/google/uuid"
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

func TestNewPlayerFromRequest(t *testing.T) {
	req := JoinQueueRequest{
		Username: "testUser",
		Flag:     "🏴",
	}

	player := NewPlayerFromRequest(req)

	assert.Equal(t, req.Username, player.Username)
	assert.Equal(t, req.Flag, player.Flag)
}

func TestGameStateMarshalling(t *testing.T) {

	// Set up a deterministic rng for UUID and maze generation

	var seed int64 = 123

	uuid.SetRand(rand.New(rand.NewSource(seed)))

	uuidStrings := []string{
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
	}

	uuid.SetRand(rand.New(rand.NewSource(seed)))

	testsCases := []struct {
		name     string
		setup    func(*GameState)
		expected string
		wantErr  bool
	}{
		{
			name:     "game state with empty player list",
			setup:    func(gs *GameState) {},
			expected: fmt.Sprintf(`{"id":"%s", "players":[%s], "seed":%v}`, uuidStrings[0], "", seed),
			wantErr:  false,
		},
		{
			name: "game state with single player",
			setup: func(gs *GameState) {
				player := NewPlayer("player1", "US")
				player.Level = 5
				gs.Players.Set(player.Id, player)
			},
			expected: fmt.Sprintf(`{"id":"%s","seed":%v, "players":[{"active":false,"flag":"US","id":"%s","level":5,"position":{"x":-1000,"y":-1000},"rotation":0,"username":"player1"}]}`, uuidStrings[1], seed, uuidStrings[2]),
			wantErr:  false,
		},
		{
			name: "game state with multiple players",
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
			expected: fmt.Sprintf(`{"id":"%s","seed":%v,"players":[{"id":"%s","active":false,"username":"player1","flag":"US","level":5,"position":{"x":-1000,"y":-1000},"rotation":0},{"id":"%s","active":false,"username":"player2","flag":"UK","level":3,"position":{"x":-1000,"y":-1000},"rotation":0},{"id":"%s","active":false,"username":"player3","flag":"FR","level":7,"position":{"x":-1000,"y":-1000},"rotation":0}]}`, uuidStrings[3], seed, uuidStrings[4], uuidStrings[5], uuidStrings[6]),
			wantErr:  false,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewGameState(seed)
			tc.setup(gs)

			bytes, err := json.Marshal(gs)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.JSONEq(t, tc.expected, string(bytes))
			}
		})
	}
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
