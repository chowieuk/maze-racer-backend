package main

import (
	"fmt"
	"time"
)

// Broadcaster defines the interface for different game state broadcasting strategies
type Broadcaster interface {
	Start(game *BaseGame)
	Stop()
}

// BaseBroadcaster provides common broadcasting functionality
type BaseBroadcaster struct {
	game     *BaseGame
	ticker   *time.Ticker
	stopChan chan struct{}
}

func NewBaseBroadcaster() *BaseBroadcaster {
	return &BaseBroadcaster{
		stopChan: make(chan struct{}),
	}
}

func (b *BaseBroadcaster) Stop() {
	if b.ticker != nil {
		b.ticker.Stop()
	}
	close(b.stopChan)
}

// SprintBroadcaster implements sprint game broadcasting
type SprintBroadcaster struct {
	*BaseBroadcaster
	roundLength time.Duration
}

func NewSprintBroadcaster(roundLength time.Duration) *SprintBroadcaster {
	return &SprintBroadcaster{
		BaseBroadcaster: NewBaseBroadcaster(),
		roundLength:     roundLength,
	}
}

func (sb *SprintBroadcaster) Start(game *BaseGame) {
	sb.game = game
	sb.ticker = time.NewTicker(game.tickrate)
	roundTimer := time.NewTimer(sb.roundLength)
	startTime := time.Now()
	game.State.StartTime = startTime.UnixMilli()

	// Send initial state
	if err := game.broadcastInitialState(); err != nil {
		fmt.Println("error broadcasting initial state:", err)
		return
	}

	for {
		select {
		case <-sb.stopChan:
			return
		case <-game.ctx.Done():
			return
		case <-roundTimer.C:
			if err := game.broadcastResult(); err != nil {
				fmt.Println("error broadcasting result:", err)
			}
			return
		case <-sb.ticker.C:
			if err := game.broadcastUpdate(); err != nil {
				fmt.Println("error broadcasting update:", err)
			}
		}
	}
}

// RaceBroadcaster implements race game broadcasting
type RaceBroadcaster struct {
	*BaseBroadcaster
	levelTarget int
}

func NewRaceBroadcaster(levelTarget int) *RaceBroadcaster {
	return &RaceBroadcaster{
		BaseBroadcaster: NewBaseBroadcaster(),
		levelTarget:     levelTarget,
	}
}

func (rb *RaceBroadcaster) Start(game *BaseGame) {
	rb.game = game
	rb.ticker = time.NewTicker(game.tickrate)
	startTime := time.Now()
	game.State.StartTime = startTime.UnixMilli()

	if err := game.broadcastInitialState(); err != nil {
		fmt.Println("error broadcasting initial state:", err)
		return
	}

	for {
		select {
		case <-rb.stopChan:
			return
		case <-game.ctx.Done():
			return
		case <-rb.ticker.C:
			if game.State.MaxLevel > rb.levelTarget {
				if err := game.broadcastResult(); err != nil {
					fmt.Println("error broadcasting result:", err)
				}
				return
			}
			if err := game.broadcastUpdate(); err != nil {
				fmt.Println("error broadcasting update:", err)
			}
		}
	}
}

// DefaultBroadcaster implements basic game broadcasting
type DefaultBroadcaster struct {
	*BaseBroadcaster
}

func NewDefaultBroadcaster() *DefaultBroadcaster {
	return &DefaultBroadcaster{
		BaseBroadcaster: NewBaseBroadcaster(),
	}
}

func (db *DefaultBroadcaster) Start(game *BaseGame) {
	db.game = game
	db.ticker = time.NewTicker(game.tickrate)
	
	if err := game.broadcastInitialState(); err != nil {
		fmt.Println("error broadcasting initial state:", err)
		return
	}

	for {
		select {
		case <-db.stopChan:
			return
		case <-game.ctx.Done():
			return
		case <-db.ticker.C:
			if err := game.broadcastUpdate(); err != nil {
				fmt.Println("error broadcasting update:", err)
			}
		}
	}
}
