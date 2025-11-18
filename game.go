package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type Game interface {
	BroadcastState()
	RunListeners()
	Cleanup()
	CheckAllPlayersReady() bool
	StartCountdown()
	GetID() string
	GetMode() GameMode
	GetMaxLevel() int
	SetMaxLevel(int)
	Add() chan<- *Client
	Remove() chan<- *Client
	Context() context.Context
	broadcastMessage([]byte)
}

// SprintGame represents a sixty second sprint maze racer game
type SprintGame struct {
	*BaseGame
	roundLength time.Duration
}

// RaceGame represents a new race to ten maze racer game
type RaceGame struct {
	*BaseGame
	levelTarget int
}

// BaseGame represents a maze racer game
type BaseGame struct {
	id            string
	tickrate      time.Duration
	Mode          GameMode
	State         *GameState
	Clients       map[*Client]bool
	add           chan *Client
	remove        chan *Client
	Broadcast     chan []byte
	ctx           context.Context
	cancel        context.CancelFunc
	countdownDone chan struct{}
	broadcaster   Broadcaster
}

// NewGame instantiates a new base game
func NewGame(mode GameMode, tickrate time.Duration) *BaseGame {
	seed := rand.Int64()
	ctx, cancel := context.WithCancel(context.Background())
	id := gonanoid.Must()
	bg := &BaseGame{
		id:            id,
		tickrate:      tickrate,
		Mode:          mode,
		State:         NewGameState(seed), // temporary seed
		Clients:       make(map[*Client]bool),
		add:           make(chan *Client),
		remove:        make(chan *Client),
		Broadcast:     make(chan []byte),
		ctx:           ctx,
		cancel:        cancel,
		countdownDone: make(chan struct{}),
	}
	bg.broadcaster = NewDefaultBroadcaster() // default broadcaster
	return bg
}

func NewSprintGame(tickrate time.Duration, roundLength time.Duration) Game {
	baseGame := NewGame(ModeSprint, tickrate)
	sprintGame := &SprintGame{
		BaseGame:    baseGame,
		roundLength: roundLength,
	}
	baseGame.broadcaster = NewSprintBroadcaster(roundLength)
	return sprintGame
}

func NewRaceGame(tickrate time.Duration, levelTarget int) Game {
	baseGame := NewGame(ModeRace, tickrate)
	raceGame := &RaceGame{
		BaseGame:    baseGame,
		levelTarget: levelTarget,
	}
	baseGame.broadcaster = NewRaceBroadcaster(levelTarget)
	return raceGame
}

func (g *BaseGame) broadcastMessage(message []byte) {
	for client := range g.Clients {
		select {
		case <-client.ctx.Done():
			g.remove <- client
		default:
			select {
			case client.send <- message:
			default:
				g.remove <- client
			}
		}
	}
}

func (g *BaseGame) broadcastInitialState() error {
	// Set initial start time
	g.State.StartTime = time.Now().UnixMilli()

	// Create and send initial state message
	initialMsg, err := g.State.AsUpdateMessage()
	if err != nil {
		return fmt.Errorf("error creating initial state message: %v", err)
	}
	g.Broadcast <- initialMsg

	// Clear start time for subsequent updates
	// g.State.StartTime = 0

	return nil
}

func (g *BaseGame) broadcastResult() error {
	result := g.State.GetRoundResult()
	msg, err := CreateResponseBytes(RespRoundResult, result)
	if err != nil {
		return fmt.Errorf("error creating round result message: %v", err)
	}

	g.Broadcast <- msg
	slog.Info("round completed",
		"game_id", g.id,
		"result", result)

	return nil
}

func (g *BaseGame) broadcastUpdate() error {
	msg, err := g.State.AsUpdateMessage()
	if err != nil {
		return fmt.Errorf("error creating state update message: %v", err)
	}
	g.Broadcast <- msg
	return nil
}

// BroadcastState starts the broadcasting - this is the public interface
func (g *BaseGame) BroadcastState() {
	slog.Info("starting game broadcast", "game_id", g.id)
	g.broadcaster.Start(g)
}

func (g *BaseGame) RunListeners() {
	defer g.cancel()

	countdownStarted := false

	// Phase 1: Countdown
	for {
		select {
		case <-g.ctx.Done():
			return
		case client := <-g.add:
			client.activeGame = g
			g.Clients[client] = true
			client.player.Active = true
			g.State.Players.Set(client.player.Id, client.player)

			if len(g.Clients) >= 2 && !countdownStarted {
				countdownStarted = true
				go g.StartCountdown()
			}

		case client := <-g.remove:
			if g.Clients[client] {
				delete(g.Clients, client)
				g.State.Players.Del(client.player.Id)
			}

			if len(g.Clients) < 2 && countdownStarted {
				slog.Info("game orphaned during countdown, sending cancel message to remaining client")

				msg := MustCreateResponseBytes(RespGameCancelled, struct{}{})

				for remainingClient := range g.Clients {
					remainingClient.send <- msg
				}

				g.Cleanup()
				return
			}

		case <-g.countdownDone:
			for client := range g.Clients {
				client.SetStatus(StatusInGame)
			}
			go g.BroadcastState()
			goto GamePhase

		case message := <-g.Broadcast:
			g.broadcastMessage(message)
		}
	}

	// Phase 2: Game Running
GamePhase:
	for {
		select {
		case <-g.ctx.Done():
			return
		case client := <-g.add:
			slog.Warn("client attempted to join running game", "client", client)
			msg := MustCreateResponseBytes(RespJoinRunningGame, struct{}{})
			client.send <- msg
		case client := <-g.remove:
			if g.Clients[client] {
				delete(g.Clients, client)
				g.State.Players.Del(client.player.Id)

				if len(g.Clients) < 2 {
					// TODO: some kind of game aborted handler?
					// TODO: what do we do with the final player?
					slog.Info("game ended due to insufficient players")

					msg := MustCreateResponseBytes(RespGameCancelled, struct{}{})

					for client := range g.Clients {
						client.send <- msg
					}
					g.Cleanup()
					return
				}
			}
		case message := <-g.Broadcast:
			g.broadcastMessage(message)
		}
	}
}

func (g *BaseGame) Cleanup() {
	g.cancel()

	for client := range g.Clients {
		g.State.Players.Del(client.player.Id)
		client.activeGame = nil
		delete(g.Clients, client)
	}

	// close(g.Add)
	// close(g.Remove)
	// close(g.Broadcast)
}

func (g *BaseGame) CheckAllPlayersReady() bool {
	for client := range g.Clients {
		if client.Status() != StatusReady {
			return false
		}
	}
	slog.Info("all players ready, adjusting countdown")
	return true
}

func (g *BaseGame) StartCountdown() {
	const (
		defaultCountdown = 30 * time.Second
		readyCountdown   = 5 * time.Second
	)

	confirmMsg := MustCreateResponseBytes(RespGameConfirmed, GameConfirmedResponse{
		GameID: g.id,
	})

	for client := range g.Clients {
		client.send <- confirmMsg
		client.SetStatus(StatusConfirming)
	}

	countdown := defaultCountdown
	ticker := time.NewTicker(time.Second)
	slog.Info("starting countdown", "duration", defaultCountdown)

	go func() {
		defer ticker.Stop()
		timeLeft := countdown
		for {
			select {
			case <-g.ctx.Done():
				return
			case <-ticker.C:
				timeLeft -= time.Second

				// Broadcast remaining time to clients
				msg, _ := CreateResponseBytes(RespSecondsToNextRoundStart, timeLeft.Seconds())
				g.Broadcast <- msg

				if timeLeft > readyCountdown && g.CheckAllPlayersReady() {
					timeLeft = readyCountdown
				}

				if timeLeft <= 0 {
					close(g.countdownDone)
					return
				}
			}
		}
	}()
}

// GetID returns the given game id
func (g *BaseGame) GetID() string {
	return g.id
}

// GetMode returns the current GameMode for the game
func (g *BaseGame) GetMode() GameMode {
	return g.Mode
}

func (g *BaseGame) GetMaxLevel() int {
	return g.State.MaxLevel
}

func (g *BaseGame) SetMaxLevel(level int) {
	g.State.MaxLevel = level
}

func (g *BaseGame) Add() chan<- *Client {
	return g.add
}

func (g *BaseGame) Remove() chan<- *Client {
	return g.remove
}

func (g *BaseGame) Context() context.Context {
	return g.ctx
}
