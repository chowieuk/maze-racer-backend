package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// GameMode represents the available game modes
type GameMode string

const (
	ModeSprint GameMode = "sprint"
	ModeRace   GameMode = "race"
)

// Matchmaker handles player queuing and game creation
type Matchmaker struct {
	tickrate time.Duration
	// Queues for head-to-head games
	sprintQueue []*Client
	raceQueue   []*Client
	// Track active head-to-head games
	headToHeadGames CMap[string, *Game]
}

// NewMatchmaker creates a new matchmaker instance
// All spawned games will use the provided tickrate
func NewMatchmaker(tickrate time.Duration) *Matchmaker {
	return &Matchmaker{
		tickrate:        tickrate,
		sprintQueue:     make([]*Client, 0),
		raceQueue:       make([]*Client, 0),
		headToHeadGames: NewMutexMap[string, *Game](),
	}
}

// AddToQueue adds a player to the queue for head-to-head games
func (m *Matchmaker) AddToQueue(c *Client, mode GameMode) error {

	switch mode {
	case ModeSprint:

		m.sprintQueue = append(m.sprintQueue, c)
		fmt.Printf("added %s to %s queue\n", c.player.Username, mode)

		if len(m.sprintQueue) >= 2 {
			fmt.Printf("Two clients in %v queue, creating new game\n", mode)

			client1 := m.sprintQueue[0]
			client2 := m.sprintQueue[1]

			game := NewGame(ModeRace, m.tickrate)

			go game.RunListeners()

			game.Add <- client1
			game.Add <- client2

			go game.BroadcastState()

			m.headToHeadGames.Set(game.GetID(), game)
			m.sprintQueue = m.sprintQueue[2:]
		}

	case ModeRace:

		m.raceQueue = append(m.raceQueue, c)
		fmt.Printf("added %s to %s queue\n", c.player.Username, mode)

		if len(m.raceQueue) >= 2 {
			fmt.Printf("Two clients in %s queue, creating new game\n", mode)

			client1 := m.raceQueue[0]
			client2 := m.raceQueue[1]

			game := NewGame(ModeRace, m.tickrate)

			go game.RunListeners()

			game.Add <- client1
			game.Add <- client2

			go game.BroadcastState()

			m.headToHeadGames.Set(game.GetID(), game)
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
		m.sprintQueue = m.sprintQueue[:1]
		return nil
	}

	inRace := slices.Contains(m.raceQueue, c)

	if inRace {
		m.raceQueue = m.raceQueue[:1]
		return nil
	}

	return fmt.Errorf("client not found in queue")
}

// Game represents a maze racer game
type Game struct {
	id        string
	tickrate  time.Duration
	Mode      GameMode
	State     *GameState
	Clients   map[*Client]bool
	Add       chan *Client
	Remove    chan *Client
	Broadcast chan []byte
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewGame instantiates a new game

func NewGame(mode GameMode, tickrate time.Duration) *Game {
	ctx, cancel := context.WithCancel(context.Background())
	return &Game{
		id:        uuid.New().String(),
		tickrate:  tickrate,
		Mode:      mode,
		State:     NewGameState(123), // temporary seed
		Clients:   make(map[*Client]bool),
		Add:       make(chan *Client),
		Remove:    make(chan *Client),
		Broadcast: make(chan []byte),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (g *Game) BroadcastState() {
	ticker := time.NewTicker(g.tickrate)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			msg, err := g.State.AsUpdateMessage()
			if err != nil {
				fmt.Println("error marshalling state: ", err)
				continue
			}
			g.Broadcast <- msg
		}
	}
}

func (g *Game) broadcastMessage(message []byte) {
	for client := range g.Clients {
		select {
		case <-client.ctx.Done():
			g.Remove <- client
		default:
			select {
			case client.send <- message:
			default:
				g.Remove <- client
			}
		}
	}
}

func (g *Game) RunListeners() {
	defer g.cancel()
	for {
		select {
		case <-g.ctx.Done():
			return

		case client := <-g.Add:

			client.activeGame = g
			g.Clients[client] = true
			client.player.Active = true
			g.State.Players.Set(client.player.Id, client.player)

		case client := <-g.Remove:
			if g.Clients[client] {
				delete(g.Clients, client)
				g.State.Players.Del(client.player.Id)
			}

		case message := <-g.Broadcast:
			g.broadcastMessage(message)
		}
	}
}

// GetID returns the given game id
func (g *Game) GetID() string {
	return g.id
}

// Client represents a connected websocket client
type Client struct {
	player     *Player
	activeGame *Game
	mm         *Matchmaker
	ws         *websocket.Conn
	send       chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
}

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

// StartReading starts the read pump for the client
func (cl *Client) StartReading() {
	defer cl.Cleanup()
	for {
		_, msg, err := cl.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				fmt.Println("unexpected read error: ", err)
			} else {
				fmt.Println("received close message: ", err)
			}
			break
		}

		var bMsg BaseMessage
		err = json.Unmarshal(msg, &bMsg)
		if err != nil {
			fmt.Printf("error unmarshalling %s: %v\n", msg, err)
			continue
		}

		switch bMsg.Type {
		case ReqJoinQueue:
			msg, err := ParseMessage[JoinQueueRequest](bMsg)
			if err != nil {
				fmt.Printf("error parsing %s: %v\n", bMsg.Type, err)
				continue
			}
			cl.HandleJoinQueue(msg)

		case ReqLeaveQueue:
			msg, err := ParseMessage[LeaveQueueRequest](bMsg)
			if err != nil {
				fmt.Printf("error parsing %s: %v\n", bMsg.Type, err)
				continue
			}
			cl.HandleLeaveQueue(msg)

		case ReqPlayerUpdate:
			msg, err := ParseMessage[PlayerUpdateRequest](bMsg)
			if err != nil {
				fmt.Printf("error parsing %s: %v\n", bMsg.Type, err)
				continue
			}
			cl.HandlePlayerUpdate(msg)

		default:
			fmt.Printf("unknown message: %s\n", bMsg)
		}

	}
}

func (cl *Client) HandleJoinQueue(req *JoinQueueRequest) {
	fmt.Println("Received join request: ", req.GameMode)
	cl.mm.AddToQueue(cl, req.GameMode)
}
func (cl *Client) HandleLeaveQueue(req *LeaveQueueRequest) {
	fmt.Println("Received leave request")
	cl.mm.RemoveFromQueue(cl)
}
func (cl *Client) HandlePlayerUpdate(req *PlayerUpdateRequest) {
	cl.player.Level = req.Level
	cl.player.Position = req.Position
	cl.player.Rotation = req.Rotation
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
	if cl.activeGame != nil {
		cl.activeGame.Remove <- cl
		cl.activeGame = nil
		cl.player.Active = false
	}
	close(cl.send)
	cl.ws.Close()
	fmt.Println("Cleaned up client: ", cl.player.Username)
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
			log.Printf("websocket upgrade error: %v", err)
			return
		}

		// Create player and client instances
		player := NewPlayer(playerName, playerFlag)
		client := NewClient(ws, player, mm)

		fmt.Printf("New connection: %v, %v\n", client.player.Username, client.player.Flag)

		resp, err := CreateMessageBytes(&ConnectedResponse{
			PlayerID: player.Id,
		})

		if err != nil {
			fmt.Println("error creating connection confirmation: ", err)
		}

		err = ws.WriteMessage(1, resp)

		if err != nil {
			fmt.Println("error writing connection confirmation: ", err)
		}

		// Start client routines

		go client.StartWriting()
		go client.StartReading()
	}
}

func main() {
	mm := NewMatchmaker(time.Second / 60)
	wsHandler := NewWebsocketHandler(mm)
	http.HandleFunc("/ws", wsHandler)

	fmt.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
