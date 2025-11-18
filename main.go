package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog"
)

// GameMode represents the available game modes
type GameMode string

const (
	ModeSprint        GameMode      = "sprint"
	ModeRace          GameMode      = "race"
	ServerTickrate    time.Duration = time.Second / 30
	SprintRoundLength time.Duration = 60 * time.Second
	RaceLevelTarget   int           = 10
)

// Matchmaker handles player queuing and game creation
type Matchmaker struct {
	tickrate time.Duration
	// Queues for head-to-head games
	sprintQueue []*Client
	raceQueue   []*Client
	// Track active head-to-head games
	headToHeadGames CMap[string, Game]
	// Track active challenges
	activeChallenges CMap[string, GameMode]
}

// NewMatchmaker creates a new matchmaker instance
// All spawned games will use the provided tickrate
func NewMatchmaker(tickrate time.Duration) *Matchmaker {
	return &Matchmaker{
		tickrate:         tickrate,
		sprintQueue:      make([]*Client, 0),
		raceQueue:        make([]*Client, 0),
		headToHeadGames:  NewMutexMap[string, Game](),
		activeChallenges: NewMutexMap[string, GameMode](),
	}
}

// AddToQueue adds a player to the queue for head-to-head games
func (m *Matchmaker) AddToQueue(c *Client, mode GameMode) error {

	switch mode {
	case ModeSprint:

		m.sprintQueue = append(m.sprintQueue, c)
		slog.Info("added player to queue",
			"player", c.player.Username,
			"queue", mode)

		queueJoined, err := CreateResponseBytes(RespQueueJoined, QueueJoinedResponse{
			Queue: mode,
		})

		if err != nil {
			return err
		}

		c.send <- queueJoined

		if len(m.sprintQueue) >= 2 {
			slog.Info("creating new game",
				"queue", mode,
				"players", 2)

			client1 := m.sprintQueue[0]
			client2 := m.sprintQueue[1]

			game := NewSprintGame(m.tickrate, SprintRoundLength)
			m.registerGame(game)

			go game.RunListeners()

			game.Add() <- client1
			game.Add() <- client2

			m.sprintQueue = m.sprintQueue[2:]
		}

	case ModeRace:

		m.raceQueue = append(m.raceQueue, c)
		slog.Info("added player to queue",
			"player", c.player.Username,
			"queue", mode)

		queueJoined, err := CreateResponseBytes(RespQueueJoined, QueueJoinedResponse{
			Queue: mode,
		})

		if err != nil {
			return err
		}

		c.send <- queueJoined

		if len(m.raceQueue) >= 2 {
			slog.Info("creating new game",
				"queue", mode,
				"players", 2)

			client1 := m.raceQueue[0]
			client2 := m.raceQueue[1]

			game := NewRaceGame(m.tickrate, RaceLevelTarget)
			m.registerGame(game)

			go game.RunListeners()

			game.Add() <- client1
			game.Add() <- client2

			m.raceQueue = m.raceQueue[2:]
		}
	default:
		return fmt.Errorf("unrecognized queue: %v", mode)
	}

	return nil
}

// RemoveFromQueue removes a player from any queue they're in
func (m *Matchmaker) RemoveFromQueue(c *Client) error {

	inSprint := slices.Contains(m.sprintQueue, c)

	if inSprint {
		queueLeft, err := CreateResponseBytes(RespQueueLeft, QueueLeftResponse{
			Queue: ModeSprint,
		})
		if err != nil {
			return fmt.Errorf("error creating response: %v", err)
		}
		c.send <- queueLeft
		m.sprintQueue = m.sprintQueue[:1]
		return nil
	}

	inRace := slices.Contains(m.raceQueue, c)

	if inRace {
		queueLeft, err := CreateResponseBytes(RespQueueLeft, QueueLeftResponse{
			Queue: ModeRace,
		})
		if err != nil {
			return fmt.Errorf("error creating response: %v", err)
		}
		c.send <- queueLeft
		m.raceQueue = m.raceQueue[:1]
		return nil
	}

	return fmt.Errorf("client not found in queue")
}

// registerGame adds a game to the matchmaker and sets up context-based cleanup
func (m *Matchmaker) registerGame(game Game) {
	m.headToHeadGames.Set(game.GetID(), game)
	slog.Info("added game to matchmaker", "game_id", game.GetID())

	// Start a goroutine that waits for the game's context to be cancelled
	go func() {
		<-game.Context().Done()
		m.headToHeadGames.Del(game.GetID())
		m.activeChallenges.Del(game.GetID())
		slog.Info("removed game from matchmaker", "game_id", game.GetID())
	}()
}

// CreateChallengeGame creates a challenge game and adds a player to it
func (m *Matchmaker) CreateChallengeGame(c *Client, mode GameMode) error {
	var game Game
	switch mode {
	case ModeSprint:
		game = NewSprintGame(m.tickrate, SprintRoundLength)
	case ModeRace:
		game = NewRaceGame(m.tickrate, RaceLevelTarget)
	default:
		return fmt.Errorf("invalid game mode")
	}

	m.registerGame(game)
	go game.RunListeners()
	game.Add() <- c
	m.activeChallenges.Set(game.GetID(), mode)
	createdMsg := MustCreateResponseBytes(RespChallengeCreated, ChallengeCreatedResponse{
		ChallengeID: game.GetID(),
	})
	c.send <- createdMsg
	return nil
}

// ChallengeActive responds true if a challenge is active
func (m *Matchmaker) ChallengeActive(challengeID string) (GameMode, bool) {
	return m.activeChallenges.Get(challengeID)
}

// AcceptChallenge adds a given client to a waiting challenge game
func (m *Matchmaker) AcceptChallenge(c *Client, challengeID string) error {

	if game, ok := m.headToHeadGames.Get(challengeID); !ok {
		return fmt.Errorf("challenge id not found: %v", challengeID)
	} else {
		m.activeChallenges.Del(challengeID)
		game.Add() <- c
		return nil
	}
}

// Client represents a connected websocket client
type Client struct {
	player     *Player
	status     ClientStatus
	activeGame Game
	mm         *Matchmaker
	ws         *websocket.Conn
	send       chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
}

type ClientStatus string

const (
	StatusQueued     ClientStatus = "queued"
	StatusConfirming ClientStatus = "confirming"
	StatusReady      ClientStatus = "ready"
	StatusInGame     ClientStatus = "in_game"
	StatusEndGame    ClientStatus = "end_game"
)

// NewClient instantiates a new client for a websocket connection
func NewClient(ws *websocket.Conn, p *Player, mm *Matchmaker) *Client {
	ctx, cancel := context.WithCancel(context.TODO())
	c := &Client{
		player:     p,
		activeGame: nil,
		mm:         mm,
		ws:         ws,
		send:       make(chan []byte, 256),
		ctx:        ctx,
		cancel:     cancel,
	}
	return c
}

func (cl *Client) Status() ClientStatus {
	return cl.status
}

func (cl *Client) SetStatus(cs ClientStatus) {
	cl.status = cs
}

// StartReading starts the read pump for the client
func (cl *Client) StartReading() {
	defer cl.Cleanup()
	for {
		_, msg, err := cl.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				slog.Error("unexpected read error", "error", err)
			} else {
				slog.Info("received close message", "error", err)
			}
			break
		}

		var bMsg BaseMessage
		err = json.Unmarshal(msg, &bMsg)
		if err != nil {
			slog.Error("error unmarshalling message",
				"message", string(msg),
				"error", err)
			continue
		}

		switch bMsg.Type {
		case ReqJoinQueue:
			msg, err := ParseMessage[JoinQueueRequest](bMsg)
			if err != nil {
				slog.Error("error parsing message",
					"type", bMsg.Type,
					"payload", string(bMsg.Payload),
					"error", err)
				continue
			}
			cl.HandleJoinQueue(msg)

		case ReqLeaveQueue:
			msg, err := ParseMessage[LeaveQueueRequest](bMsg)
			if err != nil {
				slog.Error("error parsing message",
					"type", bMsg.Type,
					"error", err)
				continue
			}
			cl.HandleLeaveQueue(msg)

		case ReqPlayerUpdate:
			msg, err := ParseMessage[PlayerUpdateRequest](bMsg)
			if err != nil {
				slog.Error("error parsing message",
					"type", bMsg.Type,
					"error", err)
				continue
			}
			cl.HandlePlayerUpdate(msg)

		case ReqPlayerReady:
			_, err := ParseMessage[PlayerReadyRequest](bMsg)
			if err != nil {
				fmt.Printf("error parsing %s: %v\n", bMsg.Type, err)
				continue
			}
			slog.Info("received ready request")
			cl.SetStatus(StatusReady)

		case ReqCreateChallenge:
			msg, err := ParseMessage[CreateChallengeRequest](bMsg)
			if err != nil {
				slog.Error("error parsing message",
					"type", bMsg.Type,
					"payload", string(bMsg.Payload),
					"error", err)
				continue
			}
			cl.HandleCreateChallenge(msg)

		case ReqAcceptChallenge:
			msg, err := ParseMessage[AcceptChallengeRequest](bMsg)
			if err != nil {
				slog.Error("error parsing message",
					"type", bMsg.Type,
					"payload", string(bMsg.Payload),
					"error", err)
				continue
			}
			cl.HandleAcceptChallenge(msg)

		default:
			slog.Warn("received unknown message", "message", bMsg)
		}

	}
}

func (cl *Client) HandleJoinQueue(req *JoinQueueRequest) {
	slog.Info("received join request", "gameMode", req.GameMode)
	cl.mm.AddToQueue(cl, req.GameMode)
}

func (cl *Client) HandleLeaveQueue(req *LeaveQueueRequest) {
	slog.Info("received leave request")
	cl.mm.RemoveFromQueue(cl)
}

func (cl *Client) HandlePlayerUpdate(req *PlayerUpdateRequest) {
	cl.player.Level = req.Level
	cl.player.Position = req.Position
	cl.player.Rotation = req.Rotation
	if cl.activeGame != nil {
		if req.Level > cl.activeGame.GetMaxLevel() {
			cl.activeGame.SetMaxLevel(req.Level)
		}
	}
}

func (cl *Client) HandleCreateChallenge(req *CreateChallengeRequest) {
	slog.Info("received create challenge request")
	cl.mm.CreateChallengeGame(cl, req.GameMode)
}

func (cl *Client) HandleAcceptChallenge(req *AcceptChallengeRequest) {
	slog.Info("received accept challenge request")
	err := cl.mm.AcceptChallenge(cl, req.ChallengeID)
	if err != nil {
		slog.Warn("error accepting challenge", "error", err)
		msg := MustCreateResponseBytes(RespChallengeStale, struct{}{})
		cl.send <- msg
	}
}

// StartWriting starts the write pump for the client
func (cl *Client) StartWriting() {
	defer cl.ws.Close()
	for {
		select {
		case <-cl.ctx.Done():
			return
		case message, ok := <-cl.send:
			if !ok {
				return
			}
			err := cl.ws.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		}
	}
}

func (cl *Client) Cleanup() {
	cl.cancel()

	err := cl.mm.RemoveFromQueue(cl)

	if err != nil {
		slog.Error("failed to remove client from queue", "error", err)
	}

	if cl.activeGame != nil {
		// Send remove signal to game if it's still active
		select {
		case cl.activeGame.Remove() <- cl:
		default:
			// Game might already be cleaning up, that's ok
		}
		cl.activeGame = nil
		cl.player.Active = false
	}

	// Close send channel only if we haven't already
	if cl.send != nil {
		close(cl.send)
		cl.send = nil
	}

	cl.ws.Close()
	slog.Info("cleaned up client", "player", cl.player.Username)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewWebsocketHandler(mm *Matchmaker) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract player information from query parameters
		playerName := r.URL.Query().Get("name")
		playerFlag := r.URL.Query().Get("flag")

		// Validate required parameters
		if playerName == "" || playerFlag == "" {
			http.Error(w, "missing player_name or player_flag parameters", http.StatusBadRequest)
			return
		}

		// Upgrade HTTP connection to WebSocket
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("websocket upgrade error", "error", err)
			return
		}

		// Create player and client instances
		player := NewPlayer(playerName, playerFlag)
		client := NewClient(ws, player, mm)

		slog.Info("new connection",
			"player", client.player.Username,
			"flag", client.player.Flag)

		resp, err := CreateMessageBytes(&ConnectedResponse{
			PlayerID: player.Id,
		})

		if err != nil {
			slog.Error("error creating connection confirmation", "error", err)
		}

		err = ws.WriteMessage(1, resp)

		if err != nil {
			slog.Error("error writing connection confirmation", "error", err)
		}

		// Start client routines

		go client.StartWriting()
		go client.StartReading()
	}
}

func NewChallengeHandler(mm *Matchmaker) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract player information from query parameters
		challengeID := r.URL.Query().Get("id")

		// Validate required parameters
		if challengeID == "" {
			slog.Warn("invalid challenge accept request", "challengeID", challengeID)
			http.Error(w, fmt.Sprintf("empty challenge id: %v", challengeID), http.StatusBadRequest)
			return
		}

		if mode, ok := mm.ChallengeActive(challengeID); !ok {
			slog.Warn("tried to accept inactive challenge", "challengeID", challengeID)
			http.Error(w, fmt.Sprintf("challenge id no longer active: %v", challengeID), http.StatusBadRequest)
			return
		} else {
			slog.Info("redirecting accept challenge request", "challengeID", challengeID)
			http.Redirect(w, r, "/?challengeID="+challengeID+"&mode="+string(mode), http.StatusFound)
		}
	}
}

func main() {
	// Initialize structured logging
	zerologLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := slog.New(slogzerolog.Option{Level: slog.LevelDebug, Logger: &zerologLogger}.NewZerologHandler())
	logger = logger.
		With("release", "v1.0.0")

	slog.SetDefault(logger)
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000" // Default port if not specified
	}

	mm := NewMatchmaker(ServerTickrate)

	wsHandler := NewWebsocketHandler(mm)
	challengeHandler := NewChallengeHandler(mm)

	// API routes
	http.HandleFunc("/api/ws", wsHandler)
	http.HandleFunc("/api/challenge", challengeHandler)

	// Health and Readiness

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	slog.Info("server starting", "port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
