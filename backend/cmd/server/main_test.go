package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

func TestChessLibraryAcceptsCoordinateMove(t *testing.T) {
	board := chess.NewGame()
	move, err := chess.UCINotation{}.Decode(board.Position(), "e2e4")
	if err != nil {
		t.Fatalf("expected coordinate move to decode: %v", err)
	}

	if err := board.Move(move); err != nil {
		t.Fatalf("expected coordinate move to be legal: %v", err)
	}

	if got := colorToken(board.Position().Turn()); got != "b" {
		t.Fatalf("expected black to move after e2e4, got %q", got)
	}
}

func TestCoordinateMoveOmitsPromotionUnlessPawnReachesBackRank(t *testing.T) {
	regular := coordinateMove(movePayload{From: "e2", To: "e4", Promotion: "q"})
	if regular != "e2e4" {
		t.Fatalf("expected regular move without promotion, got %q", regular)
	}

	promotion := coordinateMove(movePayload{From: "e7", To: "e8", Promotion: "q"})
	if promotion != "e7e8q" {
		t.Fatalf("expected promotion suffix, got %q", promotion)
	}
}

func TestWebSocketOriginPolicy(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://chess.example.com")

	sameHost := httptest.NewRequest(http.MethodGet, "http://chess.example.com/ws", nil)
	sameHost.Host = "chess.example.com"
	sameHost.Header.Set("Origin", "https://chess.example.com")
	if !checkWebSocketOrigin(sameHost) {
		t.Fatalf("expected same host origin to be allowed")
	}

	allowedOrigin := httptest.NewRequest(http.MethodGet, "http://backend.internal/ws", nil)
	allowedOrigin.Host = "backend.internal"
	allowedOrigin.Header.Set("Origin", "https://chess.example.com")
	if !checkWebSocketOrigin(allowedOrigin) {
		t.Fatalf("expected configured origin to be allowed")
	}

	localDev := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3000/ws", nil)
	localDev.Host = "127.0.0.1:3000"
	localDev.Header.Set("Origin", "http://127.0.0.1:5173")
	if !checkWebSocketOrigin(localDev) {
		t.Fatalf("expected loopback development origin to be allowed")
	}

	crossSite := httptest.NewRequest(http.MethodGet, "http://chess.example.com/ws", nil)
	crossSite.Host = "chess.example.com"
	crossSite.Header.Set("Origin", "https://evil.example.net")
	if checkWebSocketOrigin(crossSite) {
		t.Fatalf("expected cross-site origin to be rejected")
	}
}

func TestWebSocketMatchAndMove(t *testing.T) {
	stateMu.Lock()
	waiting = nil
	games = map[string]*game{}
	stateMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(httpHandler())
	server.Listener = listener
	server.Start()
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	first := dialTestClient(t, url)
	defer first.Close()
	second := dialTestClient(t, url)
	defer second.Close()

	readTestMessage(t, first, "session:ready")
	readTestMessage(t, second, "session:ready")
	writeTestMessage(t, first, "matchmaking:join", nil)
	readTestMessage(t, first, "matchmaking:waiting")
	writeTestMessage(t, second, "matchmaking:join", nil)

	firstStart := readTestMessage(t, first, "game:start")
	secondStart := readTestMessage(t, second, "game:start")

	white := first
	if firstStart.Payload["color"] != "w" {
		white = second
	}
	if firstStart.Payload["color"] == secondStart.Payload["color"] {
		t.Fatalf("expected opposite colors, got %v and %v", firstStart.Payload["color"], secondStart.Payload["color"])
	}

	writeTestMessage(t, white, "game:move", map[string]string{
		"from":      "e2",
		"to":        "e4",
		"promotion": "q",
	})
	readTestMessage(t, first, "game:update")
	update := readTestMessage(t, second, "game:update")
	if update.Payload["turn"] != "b" {
		t.Fatalf("expected black turn after e2e4, got %v", update.Payload["turn"])
	}
}

func TestWebSocketResignEndsGame(t *testing.T) {
	stateMu.Lock()
	waiting = nil
	games = map[string]*game{}
	stateMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(httpHandler())
	server.Listener = listener
	server.Start()
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	first := dialTestClient(t, url)
	defer first.Close()
	second := dialTestClient(t, url)
	defer second.Close()

	readTestMessage(t, first, "session:ready")
	readTestMessage(t, second, "session:ready")
	writeTestMessage(t, first, "matchmaking:join", nil)
	readTestMessage(t, first, "matchmaking:waiting")
	writeTestMessage(t, second, "matchmaking:join", nil)

	firstStart := readTestMessage(t, first, "game:start")
	readTestMessage(t, second, "game:start")

	writeTestMessage(t, first, "game:resign", nil)
	firstUpdate := readTestMessage(t, first, "game:update")
	secondUpdate := readTestMessage(t, second, "game:update")
	if firstUpdate.Payload["status"] != "resignation" || secondUpdate.Payload["status"] != "resignation" {
		t.Fatalf("expected resignation status, got %v and %v", firstUpdate.Payload["status"], secondUpdate.Payload["status"])
	}
	if firstUpdate.Payload["winner"] == firstStart.Payload["color"] {
		t.Fatalf("expected resigning player not to be winner")
	}
}

func TestWebSocketBotGameRespondsToMove(t *testing.T) {
	stateMu.Lock()
	waiting = nil
	games = map[string]*game{}
	stateMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(httpHandler())
	server.Listener = listener
	server.Start()
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	player := dialTestClient(t, url)
	defer player.Close()

	readTestMessage(t, player, "session:ready")
	writeTestMessage(t, player, "bot:join", nil)
	start := readTestMessage(t, player, "game:start")
	if start.Payload["mode"] != "bot" || start.Payload["color"] != "w" {
		t.Fatalf("expected white bot game start, got %#v", start.Payload)
	}

	writeTestMessage(t, player, "game:move", map[string]string{
		"from":      "e2",
		"to":        "e4",
		"promotion": "q",
	})
	humanUpdate := readTestMessage(t, player, "game:update")
	aiUpdate := readTestMessage(t, player, "game:update")
	if humanUpdate.Payload["lastMove"] != "e2e4" {
		t.Fatalf("expected human e2e4 update, got %#v", humanUpdate.Payload)
	}
	if aiUpdate.Payload["movedBy"] != "ai" {
		t.Fatalf("expected ai response update, got %#v", aiUpdate.Payload)
	}
	if aiUpdate.Payload["turn"] != "w" {
		t.Fatalf("expected turn to return to white after ai move, got %v", aiUpdate.Payload["turn"])
	}
}

func TestAuthenticatedBotGamePersistsHistoryAndStats(t *testing.T) {
	resetAuthState()
	stateMu.Lock()
	waiting = nil
	games = map[string]*game{}
	stateMu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(httpHandler())
	server.Listener = listener
	server.Start()
	defer server.Close()

	registerRequest := httptest.NewRequest(http.MethodPost, "/auth/register", mustJSONBody(t, authPayload{
		Username: "history_player",
		Password: "correct-password",
	}))
	registerResponse := httptest.NewRecorder()
	registerHandler(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d", registerResponse.Code)
	}
	sessionCookie := firstCookie(registerResponse.Result(), "chess_session")
	if sessionCookie == "" {
		t.Fatalf("expected session cookie after register")
	}

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	header := http.Header{}
	header.Set("Cookie", "chess_session="+sessionCookie)
	player, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer player.Close()

	ready := readTestMessage(t, player, "session:ready")
	if ready.Payload["username"] != "history_player" {
		t.Fatalf("expected authenticated websocket, got %#v", ready.Payload)
	}

	writeTestMessage(t, player, "bot:join", map[string]string{"level": "easy"})
	start := readTestMessage(t, player, "game:start")
	if start.Payload["mode"] != "bot" {
		t.Fatalf("expected bot game, got %#v", start.Payload)
	}

	writeTestMessage(t, player, "game:move", map[string]string{
		"from":      "e2",
		"to":        "e4",
		"promotion": "q",
	})
	readTestMessage(t, player, "game:update")
	aiUpdate := readTestMessage(t, player, "game:update")
	if aiUpdate.Payload["movedBy"] != "ai" {
		t.Fatalf("expected ai move update, got %#v", aiUpdate.Payload)
	}

	writeTestMessage(t, player, "game:resign", nil)
	resignUpdate := readTestMessage(t, player, "game:update")
	if resignUpdate.Payload["status"] != "resignation" {
		t.Fatalf("expected resignation update, got %#v", resignUpdate.Payload)
	}

	writeTestMessage(t, player, "game:rematch", nil)
	rematchStart := readTestMessage(t, player, "game:start")
	if rematchStart.Payload["mode"] != "bot" || rematchStart.Payload["color"] != "w" {
		t.Fatalf("expected rematch to restart bot game, got %#v", rematchStart.Payload)
	}

	recentRequest := httptest.NewRequest(http.MethodGet, "/games/recent", nil)
	recentRequest.Header.Set("Cookie", "chess_session="+sessionCookie)
	recentResponse := httptest.NewRecorder()
	recentGamesHandler(recentResponse, recentRequest)
	if recentResponse.Code != http.StatusOK {
		t.Fatalf("expected recent games status 200, got %d", recentResponse.Code)
	}
	if !strings.Contains(recentResponse.Body.String(), "history_player") {
		t.Fatalf("expected recent game to include player username, got %s", recentResponse.Body.String())
	}

	statsRequest := httptest.NewRequest(http.MethodGet, "/games/stats", nil)
	statsRequest.Header.Set("Cookie", "chess_session="+sessionCookie)
	statsResponse := httptest.NewRecorder()
	gameStatsHandler(statsResponse, statsRequest)
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("expected stats status 200, got %d", statsResponse.Code)
	}
	var stats gameStats
	if err := json.Unmarshal(statsResponse.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats failed: %v", err)
	}
	if stats.Total != 1 || stats.Losses != 1 {
		t.Fatalf("expected one recorded loss after resigning bot game, got %#v", stats)
	}
}

func TestWebSocketSessionReadyIncludesAuthenticatedUser(t *testing.T) {
	resetAuthState()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(httpHandler())
	server.Listener = listener
	server.Start()
	defer server.Close()

	account := userAccount{ID: "user-1", Username: "player_one", CreatedAt: time.Now().UTC()}
	response := httptest.NewRecorder()
	startSession(response, httptest.NewRequest(http.MethodPost, "/", nil), account)
	sessionCookie := firstCookie(response.Result(), "chess_session")

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	header := http.Header{}
	header.Set("Cookie", "chess_session="+sessionCookie)
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	ready := readTestMessage(t, conn, "session:ready")
	if ready.Payload["userId"] != "user-1" || ready.Payload["username"] != "player_one" {
		t.Fatalf("expected authenticated session payload, got %#v", ready.Payload)
	}
}

func TestMetricsEndpointExposesPrometheusText(t *testing.T) {
	stateMu.Lock()
	waiting = nil
	rooms = map[string]*client{}
	games = map[string]*game{}
	stateMu.Unlock()

	statsMu.Lock()
	activeConnections = 0
	totalConnections = 2
	totalMoves = 3
	completedGames = 1
	disconnects = 1
	statsMu.Unlock()

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	metricsHandler(response, request)

	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", response.Code)
	}
	for _, metric := range []string{
		"chess_active_connections 0",
		"chess_total_connections 2",
		"chess_total_moves 3",
		"chess_completed_games 1",
		"chess_disconnects 1",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("expected metrics body to contain %q, got:\n%s", metric, body)
		}
	}
}

func TestRegisterLoginMeAndLogout(t *testing.T) {
	resetAuthState()

	registerRequest := httptest.NewRequest(http.MethodPost, "/auth/register", mustJSONBody(t, authPayload{
		Username: "player_one",
		Password: "correct-password",
	}))
	registerResponse := httptest.NewRecorder()
	registerHandler(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d", registerResponse.Code)
	}
	sessionCookie := firstCookie(registerResponse.Result(), "chess_session")
	if sessionCookie == "" {
		t.Fatalf("expected session cookie after register")
	}
	csrfCookie := firstCookie(registerResponse.Result(), "chess_csrf")
	if csrfCookie == "" {
		t.Fatalf("expected csrf cookie after register")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meRequest.Header.Set("Cookie", "chess_session="+sessionCookie)
	meResponse := httptest.NewRecorder()
	meHandler(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d", meResponse.Code)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutRequest.Header.Set("Cookie", "chess_session="+sessionCookie+"; chess_csrf="+csrfCookie)
	logoutRequest.Header.Set("X-CSRF-Token", csrfCookie)
	logoutResponse := httptest.NewRecorder()
	logoutHandler(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d", logoutResponse.Code)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", mustJSONBody(t, authPayload{
		Username: "player_one",
		Password: "correct-password",
	}))
	loginResponse := httptest.NewRecorder()
	loginHandler(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d", loginResponse.Code)
	}
}

func TestLogoutRequiresCSRFToken(t *testing.T) {
	resetAuthState()
	account := userAccount{ID: "u1", Username: "player", CreatedAt: time.Now().UTC()}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	startSession(response, request, account)
	sessionCookie := firstCookie(response.Result(), "chess_session")
	csrfCookie := firstCookie(response.Result(), "chess_csrf")

	missingHeader := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	missingHeader.Header.Set("Cookie", "chess_session="+sessionCookie+"; chess_csrf="+csrfCookie)
	missingResponse := httptest.NewRecorder()
	logoutHandler(missingResponse, missingHeader)
	if missingResponse.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf status 403, got %d", missingResponse.Code)
	}

	valid := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	valid.Header.Set("Cookie", "chess_session="+sessionCookie+"; chess_csrf="+csrfCookie)
	valid.Header.Set("X-CSRF-Token", csrfCookie)
	validResponse := httptest.NewRecorder()
	logoutHandler(validResponse, valid)
	if validResponse.Code != http.StatusOK {
		t.Fatalf("expected valid csrf status 200, got %d", validResponse.Code)
	}
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	resetAuthState()

	request := httptest.NewRequest(http.MethodPost, "/auth/register", mustJSONBody(t, authPayload{
		Username: "player_two",
		Password: "correct-password",
	}))
	response := httptest.NewRecorder()
	registerHandler(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected initial register 201, got %d", response.Code)
	}

	duplicateRequest := httptest.NewRequest(http.MethodPost, "/auth/register", mustJSONBody(t, authPayload{
		Username: "player_two",
		Password: "correct-password",
	}))
	duplicateResponse := httptest.NewRecorder()
	registerHandler(duplicateResponse, duplicateRequest)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("expected duplicate register 409, got %d", duplicateResponse.Code)
	}
}

func TestAuthRateLimit(t *testing.T) {
	resetAuthState()

	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(http.MethodPost, "/auth/login", mustJSONBody(t, authPayload{
			Username: "missing_user",
			Password: "wrong-password",
		}))
		request.RemoteAddr = "198.51.100.10:1234"
		response := httptest.NewRecorder()
		loginHandler(response, request)
		if response.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was rate limited too early", i+1)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/auth/login", mustJSONBody(t, authPayload{
		Username: "missing_user",
		Password: "wrong-password",
	}))
	request.RemoteAddr = "198.51.100.10:1234"
	response := httptest.NewRecorder()
	loginHandler(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit status 429, got %d", response.Code)
	}
}

func TestRecentGamesRequiresLoginAndReturnsUserGames(t *testing.T) {
	resetAuthState()

	account := userAccount{ID: "user-1", Username: "player_one", CreatedAt: time.Now().UTC()}
	startSessionResponse := httptest.NewRecorder()
	startSession(startSessionResponse, httptest.NewRequest(http.MethodPost, "/", nil), account)
	sessionCookie := firstCookie(startSessionResponse.Result(), "chess_session")

	historyMu.Lock()
	memoryGameHistory = []gameRecord{
		{ID: "other-game", WhiteUserID: "other-user", Mode: "bot", Moves: []string{"e2e4"}, StartedAt: time.Now().UTC(), EndedAt: time.Now().UTC()},
		{ID: "user-game", WhiteUserID: "user-1", WhiteUsername: "player_one", Mode: "bot", Moves: []string{"e2e4", "e7e5"}, Outcome: "1-0", StartedAt: time.Now().UTC(), EndedAt: time.Now().UTC()},
	}
	historyMu.Unlock()

	unauthenticated := httptest.NewRecorder()
	recentGamesHandler(unauthenticated, httptest.NewRequest(http.MethodGet, "/games/recent", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated recent games status 401, got %d", unauthenticated.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/games/recent", nil)
	request.Header.Set("Cookie", "chess_session="+sessionCookie)
	response := httptest.NewRecorder()
	recentGamesHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected recent games status 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "user-game") || strings.Contains(body, "other-game") {
		t.Fatalf("expected only signed-in user's game, got %s", body)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/games/detail?id=user-game", nil)
	detailRequest.Header.Set("Cookie", "chess_session="+sessionCookie)
	detailResponse := httptest.NewRecorder()
	gameDetailHandler(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("expected game detail status 200, got %d", detailResponse.Code)
	}
	if !strings.Contains(detailResponse.Body.String(), "user-game") {
		t.Fatalf("expected detail response to include user game, got %s", detailResponse.Body.String())
	}

	otherDetailRequest := httptest.NewRequest(http.MethodGet, "/games/detail?id=other-game", nil)
	otherDetailRequest.Header.Set("Cookie", "chess_session="+sessionCookie)
	otherDetailResponse := httptest.NewRecorder()
	gameDetailHandler(otherDetailResponse, otherDetailRequest)
	if otherDetailResponse.Code != http.StatusNotFound {
		t.Fatalf("expected other user's game detail status 404, got %d", otherDetailResponse.Code)
	}
}

func TestGameStatsReturnsUserRecordSummary(t *testing.T) {
	resetAuthState()

	account := userAccount{ID: "user-1", Username: "player_one", CreatedAt: time.Now().UTC()}
	startSessionResponse := httptest.NewRecorder()
	startSession(startSessionResponse, httptest.NewRequest(http.MethodPost, "/", nil), account)
	sessionCookie := firstCookie(startSessionResponse.Result(), "chess_session")

	historyMu.Lock()
	memoryGameHistory = []gameRecord{
		{ID: "win", WhiteUserID: "user-1", Winner: "w", Outcome: "1-0", Moves: []string{"e2e4"}, EndedAt: time.Now().UTC()},
		{ID: "loss", BlackUserID: "user-1", Winner: "w", Outcome: "1-0", Moves: []string{"e2e4", "e7e5"}, EndedAt: time.Now().UTC()},
		{ID: "draw", WhiteUserID: "user-1", Outcome: "1/2-1/2", Moves: []string{"g1f3", "g8f6", "f3g1"}, EndedAt: time.Now().UTC()},
		{ID: "other", WhiteUserID: "other-user", Winner: "w", Outcome: "1-0", Moves: []string{"e2e4"}, EndedAt: time.Now().UTC()},
	}
	historyMu.Unlock()

	request := httptest.NewRequest(http.MethodGet, "/games/stats", nil)
	request.Header.Set("Cookie", "chess_session="+sessionCookie)
	response := httptest.NewRecorder()
	gameStatsHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected stats status 200, got %d", response.Code)
	}
	var stats gameStats
	if err := json.Unmarshal(response.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats failed: %v", err)
	}
	if stats.Total != 3 || stats.Wins != 1 || stats.Draws != 1 || stats.Losses != 1 || stats.LongestGame != 3 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestAdminStatusRequiresLoginAndReturnsRuntimeStatus(t *testing.T) {
	resetAuthState()
	t.Setenv("ADMIN_USERS", "admin_user")

	stateMu.Lock()
	waiting = []*client{{id: "waiting"}}
	rooms = map[string]*client{"ABC123": {id: "room-owner"}}
	games = map[string]*game{"game-1": {id: "game-1"}}
	stateMu.Unlock()

	statsMu.Lock()
	activeConnections = 2
	totalConnections = 5
	totalMoves = 9
	completedGames = 3
	disconnects = 1
	statsMu.Unlock()

	unauthenticated := httptest.NewRecorder()
	adminStatusHandler(unauthenticated, httptest.NewRequest(http.MethodGet, "/admin/status", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated admin status 401, got %d", unauthenticated.Code)
	}

	account := userAccount{ID: "user-1", Username: "player_one", CreatedAt: time.Now().UTC()}
	startSessionResponse := httptest.NewRecorder()
	startSession(startSessionResponse, httptest.NewRequest(http.MethodPost, "/", nil), account)
	sessionCookie := firstCookie(startSessionResponse.Result(), "chess_session")

	request := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	request.Header.Set("Cookie", "chess_session="+sessionCookie)
	response := httptest.NewRecorder()
	adminStatusHandler(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin status 403, got %d", response.Code)
	}

	admin := userAccount{ID: "admin-1", Username: "admin_user", IsAdmin: true, CreatedAt: time.Now().UTC()}
	adminSessionResponse := httptest.NewRecorder()
	startSession(adminSessionResponse, httptest.NewRequest(http.MethodPost, "/", nil), admin)
	adminSessionCookie := firstCookie(adminSessionResponse.Result(), "chess_session")

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	adminRequest.Header.Set("Cookie", "chess_session="+adminSessionCookie)
	adminResponse := httptest.NewRecorder()
	adminStatusHandler(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("expected admin status 200, got %d", adminResponse.Code)
	}
	var status runtimeStatus
	if err := json.Unmarshal(adminResponse.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode runtime status failed: %v", err)
	}
	if status.ActiveConnections != 2 || status.WaitingPlayers != 1 || status.OpenRooms != 1 || status.ActiveGames != 1 || status.TotalMoves != 9 {
		t.Fatalf("unexpected runtime status: %#v", status)
	}
}

func TestReadyHandlerReportsDependencyReadiness(t *testing.T) {
	previousDB := dbPool
	previousCache := cache
	dbPool = nil
	cache = nil
	defer func() {
		dbPool = previousDB
		cache = previousCache
	}()

	response := httptest.NewRecorder()
	readyHandler(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected ready status 503 without dependencies, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "not_configured") {
		t.Fatalf("expected readiness body to explain missing dependencies, got %s", response.Body.String())
	}
}

func TestHTTPMiddlewareAddsSecurityHeadersAndJSONLogs(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)

	request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.RemoteAddr = "203.0.113.8:4567"
	response := httptest.NewRecorder()
	httpHandler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth status 401, got %d", response.Code)
	}
	for header, expected := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := response.Header().Get(header); got != expected {
			t.Fatalf("expected %s header %q, got %q", header, expected, got)
		}
	}
	output := logs.String()
	if !strings.Contains(output, `"event":"http_request"`) || !strings.Contains(output, `"status":401`) {
		t.Fatalf("expected structured request log, got %s", output)
	}
}

func TestRecoveryMiddlewareReturnsJSONError(t *testing.T) {
	handler := withSecurityHeaders(withRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Internal server error") {
		t.Fatalf("expected generic json error, got %s", response.Body.String())
	}
}

func resetAuthState() {
	authMu.Lock()
	defer authMu.Unlock()
	memoryUsers = map[string]userAccount{}
	sessions = map[string]sessionRecord{}
	rateMu.Lock()
	defer rateMu.Unlock()
	authBuckets = map[string]*rateBucket{}
	historyMu.Lock()
	defer historyMu.Unlock()
	memoryGameHistory = nil
}

func mustJSONBody(t *testing.T, payload interface{}) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body failed: %v", err)
	}
	return bytes.NewReader(raw)
}

func firstCookie(response *http.Response, name string) string {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

type testMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

func dialTestClient(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	return conn
}

func readTestMessage(t *testing.T, conn *websocket.Conn, expected string) testMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg testMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read websocket message failed: %v", err)
	}
	if msg.Type != expected {
		t.Fatalf("expected message %q, got %q", expected, msg.Type)
	}
	return msg
}

func writeTestMessage(t *testing.T, conn *websocket.Conn, messageType string, payload interface{}) {
	t.Helper()
	raw, err := json.Marshal(outboundMessage{Type: messageType, Payload: payload})
	if err != nil {
		t.Fatalf("marshal message failed: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write websocket message failed: %v", err)
	}
}
