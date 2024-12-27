package main

import (
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

func TestNewPlayerFromRequest(t *testing.T) {
	req := JoinQueueRequest{
		Username: "testUser",
		Flag:     "🏴",
	}

	player := NewPlayerFromRequest(req)

	assert.Equal(t, req.Username, player.Username)
	assert.Equal(t, req.Flag, player.Flag)
}
