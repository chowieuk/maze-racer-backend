# TODO:

- [x] sketch out game UX
- [x] sketch out API and data flow
- [x] sketch out backend game state and entity modeling
- [ ] breakup server into file components
- [ ] POC server working locally
- [ ] add new stub message handlers to client
- [ ] add new stub message handlers to server
- [ ] add new client message classes to client
- [ ] add new server message structs to server
- [ ] finalize server models schemas
- [ ] move server executable builds to a /build directory
- [ ] POC deployment to cloud service and deployment process
- [ ] add server logging control
- [ ] finalize server message schemas

  # BUGS:

- [ ] BUG:

- [ ] implement full game features:

INVESTIGATE:

- load testing client connections
- performance testing golang server performance
- performance testing websocket message latency
- ping/pong needed for clients that start to fail
- look into a proper logging library

/////////// VERSUS MENU UX ///////////

1. player chooses head to head queue or active multi ball game

H2H:

1. player chooses sprint or race mode
2. player is assigned a name and flag based on their location
3. player is shown a "waiting..." message until an opponent joins their game mode queue
4. game begins when two players have joined a queue

// TODO: expand this to include a ready message, and an associated countdown for games to begin

Multi Ball:

1. player chooses sprint or race mode
2. player is assigned a name and flag based on their location
3. player joins the active game of that game mode

/////////// GAME UX ///////////

1. a maze level is generated using the seed provided by the backend
2. player navigates a maze to complete a level
3. after a brief pause the player is spawned at the start of the next level
4. the game client renders opponents that are on the same level as the player
5. sprint mode games have rounds that conclude after a 60 second time limit
6. race mode games have rounds that conclude after a player completes level 10
7. at the end of each round, winner is declared and player's rank is displayed

/////////// MESSAGE API ///////////

Client Request Messages:
REQ_PLAYER_JOIN_QUEUE
REQ_PLAYER_LEAVE_QUEUE
REQ_PLAYER_ENTER_GAME
REQ_PLAYER_EXIT_GAME
REQ_PLAYER_UPDATE

Server Response Messages:
RESP_GAME_STATE
RESP_PLAYER_JOIN_QUEUE
RESP_PLAYER_LEAVE_QUEUE
RESP_GAME_CONFIRMED
RESP_PLAYER_ENTER_GAME
RESP_PLAYER_EXIT_GAME
RESP_PLAYER_STATE_UPDATE
RESP_SECONDS_TO_NEXT_ROUND_START
RESP_SECONDS_TO_CURRENT_ROUND_END
RESP_ROUND_RESULT

/////////// API PAYLOAD SCHEMAS ///////////

Message:
(Generic for all client and server messages)
{
messageType: <string>,
payload: <JSON>
}

/////////// MODEL SCHEMAS ///////////

Game State:
{
players: {<str_UUID>: <Player>, ...},
roundHistory: {<str_UUID>: <Round>, ...},
roundCurrent: <Round>,
roundInProgress: <bool>,
secondsToCurrentRoundEnd: <int_sec>,
secondsToNextRoundStart: <int_sec>
}

Round:
{
id: <str_UUID>,
playerIdToScore: {<str_UUID>: <int>, ...},
timeStart: <int_unix_ts>,
timeEnd: <int_unix_ts>
}

Player:
{
id: <str_UUID>,
active: <bool>,
name: <string>,
flag: <string>,
level: <int>
position: {
x: <float64>,
y: <float64>
},
rotation: <float64>
}
