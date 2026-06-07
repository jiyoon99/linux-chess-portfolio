package main

import (
	"bufio"
	"context"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notnil/chess"
	"github.com/redis/go-redis/v9"
)

type client struct {
	id         string
	conn       *websocket.Conn
	mu         sync.Mutex
	gameID     string
	color      chess.Color
	spectating bool
	watchCode  string
	userID     string
	username   string
}

type game struct {
	id         string
	board      *chess.Game
	white      *client
	black      *client
	spectators map[string]*client
	aiColor    chess.Color
	aiLevel    string
	aiEngine   string
	roomCode   string
	persisted  bool
	counted    bool
	moves      []string
	rematch    map[string]bool
	createdAt  time.Time
	mu         sync.Mutex
}

type inboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type movePayload struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion"`
}

type botJoinPayload struct {
	Level string `json:"level"`
}

type roomPayload struct {
	Code string `json:"code"`
}

type authPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userAccount struct {
	ID           string
	Username     string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
}

type sessionRecord struct {
	Account   userAccount
	ExpiresAt time.Time
}

type gameRecord struct {
	ID            string    `json:"id"`
	Mode          string    `json:"mode"`
	RoomCode      string    `json:"roomCode,omitempty"`
	Moves         []string  `json:"moves"`
	FinalFEN      string    `json:"finalFen"`
	Outcome       string    `json:"outcome"`
	Method        string    `json:"method"`
	Winner        string    `json:"winner,omitempty"`
	AILevel       string    `json:"aiLevel,omitempty"`
	AIEngine      string    `json:"aiEngine,omitempty"`
	WhiteUserID   string    `json:"whiteUserId,omitempty"`
	WhiteUsername string    `json:"whiteUsername,omitempty"`
	BlackUserID   string    `json:"blackUserId,omitempty"`
	BlackUsername string    `json:"blackUsername,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
	EndedAt       time.Time `json:"endedAt"`
}

type gameStats struct {
	Total       int    `json:"total"`
	Wins        int    `json:"wins"`
	Draws       int    `json:"draws"`
	Losses      int    `json:"losses"`
	WinRate     int    `json:"winRate"`
	LastPlayed  string `json:"lastPlayed,omitempty"`
	LongestGame int    `json:"longestGame"`
}

type adminUserSummary struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"isAdmin"`
	CreatedAt string `json:"createdAt"`
}

type runtimeStatus struct {
	ActiveConnections int                `json:"activeConnections"`
	TotalConnections  int                `json:"totalConnections"`
	WaitingPlayers    int                `json:"waitingPlayers"`
	OpenRooms         int                `json:"openRooms"`
	ActiveGames       int                `json:"activeGames"`
	CompletedGames    int                `json:"completedGames"`
	TotalMoves        int                `json:"totalMoves"`
	Disconnects       int                `json:"disconnects"`
	DB                bool               `json:"db"`
	Redis             bool               `json:"redis"`
	AIEngine          string             `json:"aiEngine"`
	ServerTime        string             `json:"serverTime"`
	UserCount         int                `json:"userCount"`
	GameCount         int                `json:"gameCount"`
	RecentUsers       []adminUserSummary `json:"recentUsers"`
}

type outboundMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: checkWebSocketOrigin,
	}

	// 실시간 게임 판단은 지연을 줄이기 위해 메모리 상태를 기준으로 처리한다.
	// Redis에는 운영 패널과 재시작 대응에 필요한 일부 상태만 복제한다.
	stateMu        sync.Mutex
	waiting        []*client
	rooms          = map[string]*client{}
	roomSpectators = map[string]map[string]*client{}
	games          = map[string]*game{}
	dbPool         *pgxpool.Pool
	cache          *redis.Client

	statsMu           sync.Mutex
	activeConnections int
	totalConnections  int
	totalMoves        int
	completedGames    int
	disconnects       int

	// 로컬 데모에서는 PostgreSQL/Redis 없이도 실행되도록 메모리 fallback을 둔다.
	// 운영 환경에서는 Redis가 설정된 경우 활성 세션을 함께 저장한다.
	authMu        sync.Mutex
	memoryUsers   = map[string]userAccount{}
	sessions      = map[string]sessionRecord{}
	errAuthFailed = errors.New("invalid username or password")

	rateMu      sync.Mutex
	authBuckets = map[string]*rateBucket{}

	historyMu         sync.Mutex
	memoryGameHistory []gameRecord
)

type rateBucket struct {
	tokens int
	reset  time.Time
}

func main() {
	initStore()
	defer closeStore()
	initCache()
	defer closeCache()

	port := serverPort()
	handler := httpHandler()
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logJSON("info", "server_start", map[string]interface{}{"addr": ":" + port})
	if err := server.ListenAndServe(); err != nil {
		logJSON("error", "server_stopped", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}

func serverPort() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return "3000"
	}
	return port
}

func httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", registerHandler)
	mux.HandleFunc("/auth/login", loginHandler)
	mux.HandleFunc("/auth/logout", logoutHandler)
	mux.HandleFunc("/auth/me", meHandler)
	mux.HandleFunc("/games/recent", recentGamesHandler)
	mux.HandleFunc("/games/stats", gameStatsHandler)
	mux.HandleFunc("/games/detail", gameDetailHandler)
	mux.HandleFunc("/games/export", gameExportHandler)
	mux.HandleFunc("/admin/status", adminStatusHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)
	mux.HandleFunc("/ws", wsHandler)

	// panic 복구와 요청 로그를 유지하면서 정상/오류 응답 모두에 보안 헤더를 적용한다.
	return withSecurityHeaders(withRecovery(withRequestLogging(mux)))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"uptime":   time.Now().UTC().Format(time.RFC3339),
		"aiEngine": aiEngineName(),
		"db":       dbPool != nil,
		"redis":    cache != nil,
	})
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	// /ready는 /health보다 엄격하다. 운영 트래픽을 받기 전 DB/Redis 응답까지 확인한다.
	status := map[string]interface{}{
		"ok":     true,
		"db":     false,
		"redis":  false,
		"checks": map[string]string{},
	}
	checks := map[string]string{}

	if dbPool == nil {
		status["ok"] = false
		checks["db"] = "not_configured"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err := dbPool.Ping(ctx)
		cancel()
		if err != nil {
			status["ok"] = false
			checks["db"] = err.Error()
		} else {
			status["db"] = true
			checks["db"] = "ok"
		}
	}

	if cache == nil {
		status["ok"] = false
		checks["redis"] = "not_configured"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err := cache.Ping(ctx).Err()
		cancel()
		if err != nil {
			status["ok"] = false
			checks["redis"] = err.Error()
		} else {
			status["redis"] = true
			checks["redis"] = "ok"
		}
	}

	status["checks"] = checks
	if status["ok"] == false {
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Prometheus가 직접 scrape하는 지표다. 별도 라이브러리 없이 노출 지표를 명확히 보여준다.
	stateMu.Lock()
	waitingCount := len(waiting)
	roomCount := len(rooms)
	activeGames := len(games)
	stateMu.Unlock()

	statsMu.Lock()
	active := activeConnections
	total := totalConnections
	moves := totalMoves
	completed := completedGames
	disconnected := disconnects
	statsMu.Unlock()

	w.Header().Set("content-type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(
		"# HELP chess_active_connections Active WebSocket connections.\n" +
			"# TYPE chess_active_connections gauge\n" +
			"chess_active_connections " + strconv.Itoa(active) + "\n" +
			"# HELP chess_total_connections Total WebSocket connections accepted.\n" +
			"# TYPE chess_total_connections counter\n" +
			"chess_total_connections " + strconv.Itoa(total) + "\n" +
			"# HELP chess_waiting_players Players currently waiting for matchmaking.\n" +
			"# TYPE chess_waiting_players gauge\n" +
			"chess_waiting_players " + strconv.Itoa(waitingCount) + "\n" +
			"# HELP chess_open_rooms Private rooms waiting for another player.\n" +
			"# TYPE chess_open_rooms gauge\n" +
			"chess_open_rooms " + strconv.Itoa(roomCount) + "\n" +
			"# HELP chess_active_games Games currently tracked in memory.\n" +
			"# TYPE chess_active_games gauge\n" +
			"chess_active_games " + strconv.Itoa(activeGames) + "\n" +
			"# HELP chess_total_moves Legal moves accepted by the server.\n" +
			"# TYPE chess_total_moves counter\n" +
			"chess_total_moves " + strconv.Itoa(moves) + "\n" +
			"# HELP chess_completed_games Games that reached a terminal result.\n" +
			"# TYPE chess_completed_games counter\n" +
			"chess_completed_games " + strconv.Itoa(completed) + "\n" +
			"# HELP chess_disconnects WebSocket disconnects observed by the server.\n" +
			"# TYPE chess_disconnects counter\n" +
			"chess_disconnects " + strconv.Itoa(disconnected) + "\n" +
			"# HELP chess_redis_enabled Redis cache availability.\n" +
			"# TYPE chess_redis_enabled gauge\n" +
			"chess_redis_enabled " + boolMetric(cache != nil) + "\n",
	))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logJSON("error", "panic_recovered", map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
					"error":  fmt.Sprint(recovered),
				})
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error."})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			return
		}
		logJSON("info", "http_request", map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"bytes":       recorder.bytes,
			"duration_ms": time.Since(start).Milliseconds(),
			"remote_ip":   clientIP(r),
		})
	})
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	n, err := recorder.ResponseWriter.Write(body)
	recorder.bytes += n
	return n, err
}

func (recorder *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := recorder.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	if recorder.status == http.StatusOK {
		recorder.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (recorder *statusRecorder) Flush() {
	if flusher, ok := recorder.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func logJSON(level string, event string, fields map[string]interface{}) {
	payload := map[string]interface{}{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"event": event,
	}
	for key, value := range fields {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf(`{"level":"error","event":"log_encode_failed","error":%q}`, err.Error())
		return
	}
	log.Print(string(raw))
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !allowAuthRequest(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many attempts. Try again later."})
		return
	}
	payload, ok := decodeAuthPayload(w, r)
	if !ok {
		return
	}
	if err := validateAuthPayload(payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	account, err := createUser(r.Context(), payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Username is already taken."})
			return
		}
		logJSON("error", "register_failed", map[string]interface{}{"error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Registration failed."})
		return
	}
	startSession(w, r, account)
	writeJSON(w, http.StatusCreated, authView(account))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !allowAuthRequest(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many attempts. Try again later."})
		return
	}
	payload, ok := decodeAuthPayload(w, r)
	if !ok {
		return
	}
	account, err := authenticateUser(r.Context(), payload)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid username or password."})
		return
	}
	startSession(w, r, account)
	writeJSON(w, http.StatusOK, authView(account))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validateCSRF(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "CSRF token required."})
		return
	}
	if cookie, err := r.Cookie("chess_session"); err == nil {
		authMu.Lock()
		delete(sessions, cookie.Value)
		authMu.Unlock()
		cacheDeleteSession(cookie.Value)
	}
	clearSessionCookie(w, r)
	clearCSRFCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	writeJSON(w, http.StatusOK, authView(account))
}

func recentGamesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	records, err := recentGames(r.Context(), account.ID, 8)
	if err != nil {
		logJSON("error", "recent_games_failed", map[string]interface{}{"user_id": account.ID, "error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not load recent games."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"games": records})
}

func gameStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	stats, err := statsForUser(r.Context(), account.ID)
	if err != nil {
		logJSON("error", "game_stats_failed", map[string]interface{}{"user_id": account.ID, "error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not load game stats."})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func gameDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	gameID := strings.TrimSpace(r.URL.Query().Get("id"))
	if gameID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing game id."})
		return
	}
	record, err := gameForUser(r.Context(), account.ID, gameID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Game not found."})
			return
		}
		logJSON("error", "game_detail_failed", map[string]interface{}{"user_id": account.ID, "game_id": gameID, "error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not load game."})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func gameExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	if strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))) != "" && strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))) != "pgn" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unsupported export format."})
		return
	}
	gameID := strings.TrimSpace(r.URL.Query().Get("id"))
	if gameID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing game id."})
		return
	}
	record, err := gameForUser(r.Context(), account.ID, gameID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Game not found."})
			return
		}
		logJSON("error", "game_export_failed", map[string]interface{}{"user_id": account.ID, "game_id": gameID, "error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not export game."})
		return
	}
	pgn, err := buildPGNExport(record)
	if err != nil {
		logJSON("error", "game_export_encode_failed", map[string]interface{}{"user_id": account.ID, "game_id": gameID, "error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not export game."})
		return
	}

	filename := fmt.Sprintf("linux-chess-%s.pgn", record.ID)
	w.Header().Set("Content-Type", "application/x-chess-pgn; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(pgn))
}

func adminStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	account, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not signed in."})
		return
	}
	if !account.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required."})
		return
	}
	writeJSON(w, http.StatusOK, currentRuntimeStatus())
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logJSON("error", "websocket_upgrade_failed", map[string]interface{}{"remote_ip": clientIP(r), "error": err.Error()})
		return
	}

	c := &client{id: randomID(12), conn: conn}
	if account, ok := currentUser(r); ok {
		c.userID = account.ID
		c.username = account.Username
	}
	recordConnectionOpened()
	send(c, "session:ready", map[string]string{"clientId": c.id, "userId": c.userID, "username": c.username})

	// WebSocket 메시지가 실시간 명령 API 역할을 한다. 메시지 타입별로 책임을 분리한다.
	for {
		var msg inboundMessage
		if err := conn.ReadJSON(&msg); err != nil {
			handleClose(c)
			return
		}

		switch msg.Type {
		case "matchmaking:join":
			handleJoin(c)
		case "room:create":
			handleCreateRoom(c)
		case "room:join":
			handleJoinRoom(c, msg.Payload)
		case "spectate:join":
			handleSpectateJoin(c, msg.Payload)
		case "bot:join":
			handleBotJoin(c, msg.Payload)
		case "game:move":
			handleMove(c, msg.Payload)
		case "game:resign":
			handleResign(c)
		case "game:rematch":
			handleRematch(c)
		case "game:analysis":
			handleAnalysis(c)
		default:
			send(c, "game:error", map[string]string{"message": "Unknown message type."})
		}
	}
}

func handleJoin(c *client) {
	stateMu.Lock()
	defer stateMu.Unlock()

	if c.gameID != "" {
		return
	}

	removeWaitingLocked(c)
	if len(waiting) == 0 {
		waiting = append(waiting, c)
		send(c, "matchmaking:waiting", nil)
		return
	}

	// 두 번째 대기자가 들어오면 매칭을 완성한다. 큐 순서가 항상 백색을 결정하지 않게 색을 섞는다.
	opponent := waiting[0]
	waiting = waiting[1:]
	if mrand.Intn(2) == 0 {
		startGameLocked(c, opponent)
		return
	}
	startGameLocked(opponent, c)
}

func startGameLocked(white *client, black *client) {
	startGameWithRoomLocked(white, black, "")
}

func startGameWithRoomLocked(white *client, black *client, roomCode string) {
	// 호출자는 이미 stateMu를 잡고 있다. 큐/방 제거와 게임 생성을 한 번에 처리해 중복 참가를 막는다.
	g := &game{
		id:         randomID(10),
		board:      chess.NewGame(),
		white:      white,
		black:      black,
		spectators: map[string]*client{},
		roomCode:   roomCode,
		moves:      []string{},
		createdAt:  time.Now().UTC(),
	}

	white.gameID = g.id
	white.color = chess.White
	black.gameID = g.id
	black.color = chess.Black
	games[g.id] = g
	cacheSetGame(g)

	send(white, "game:start", gameView(g, chess.White, nil))
	send(black, "game:start", gameView(g, chess.Black, nil))
	attachRoomSpectatorsLocked(roomCode, g)
}

func startBotGameLocked(c *client, level string) {
	g := &game{
		id:         randomID(10),
		board:      chess.NewGame(),
		white:      c,
		spectators: map[string]*client{},
		aiColor:    chess.Black,
		aiLevel:    normalizeAILevel(level),
		aiEngine:   aiEngineName(),
		moves:      []string{},
		createdAt:  time.Now().UTC(),
	}

	c.gameID = g.id
	c.color = chess.White
	games[g.id] = g
	cacheSetGame(g)
	send(c, "game:start", gameView(g, chess.White, map[string]interface{}{"mode": "bot"}))
}

func handleCreateRoom(c *client) {
	stateMu.Lock()
	defer stateMu.Unlock()

	if c.gameID != "" {
		return
	}

	removeWaitingLocked(c)
	code := uniqueRoomCodeLocked()
	rooms[code] = c
	if roomSpectators[code] == nil {
		roomSpectators[code] = map[string]*client{}
	}
	// 방 코드는 Redis에 만료 시간을 두고 기록한다. 현재 프로세스의 실제 매칭 기준은 메모리 map이다.
	cacheSetRoom(code, c.id)
	send(c, "room:created", map[string]string{"code": code})
	send(c, "room:waiting", map[string]string{"code": code})
}

func handleJoinRoom(c *client, raw json.RawMessage) {
	var payload roomPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		send(c, "game:error", map[string]string{"message": "Malformed room code."})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(payload.Code))

	stateMu.Lock()
	defer stateMu.Unlock()

	host := rooms[code]
	if host == nil || host == c {
		send(c, "game:error", map[string]string{"message": "Room not found."})
		return
	}
	delete(rooms, code)
	cacheDeleteRoom(code)
	removeWaitingLocked(c)
	startGameWithRoomLocked(host, c, code)
}

func handleSpectateJoin(c *client, raw json.RawMessage) {
	var payload roomPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		send(c, "game:error", map[string]string{"message": "Malformed room code."})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(payload.Code))

	stateMu.Lock()
	defer stateMu.Unlock()

	if c.gameID != "" {
		send(c, "game:error", map[string]string{"message": "Already in a game."})
		return
	}

	if roomSpectators[code] == nil {
		roomSpectators[code] = map[string]*client{}
	}
	if host := rooms[code]; host != nil {
		roomSpectators[code][c.id] = c
		c.spectating = true
		c.watchCode = code
		send(c, "room:watching", map[string]string{"code": code})
		return
	}

	if g := activeGameForRoomLocked(code); g != nil {
		attachSpectatorLocked(g, c, code)
		return
	}

	send(c, "game:error", map[string]string{"message": "Room not found."})
}

func attachRoomSpectatorsLocked(roomCode string, g *game) {
	if roomCode == "" || g == nil {
		return
	}
	spectators := roomSpectators[roomCode]
	if len(spectators) == 0 {
		delete(roomSpectators, roomCode)
		return
	}
	for id, spectator := range spectators {
		attachSpectatorLocked(g, spectator, roomCode)
		delete(spectators, id)
	}
	delete(roomSpectators, roomCode)
}

func attachSpectatorLocked(g *game, c *client, roomCode string) {
	if g == nil || c == nil {
		return
	}
	if g.spectators == nil {
		g.spectators = map[string]*client{}
	}
	c.gameID = g.id
	c.color = chess.NoColor
	c.spectating = true
	c.watchCode = roomCode
	g.spectators[c.id] = c
	cacheSetGame(g)
	send(c, "game:start", gameView(g, chess.NoColor, map[string]interface{}{"mode": "spectator", "spectator": true, "roomCode": roomCode}))
}

func activeGameForRoomLocked(roomCode string) *game {
	if roomCode == "" {
		return nil
	}
	for _, g := range games {
		if g != nil && g.roomCode == roomCode && g.board.Outcome() == chess.NoOutcome {
			return g
		}
	}
	return nil
}

func handleBotJoin(c *client, raw json.RawMessage) {
	var payload botJoinPayload
	_ = json.Unmarshal(raw, &payload)
	level := normalizeAILevel(payload.Level)

	stateMu.Lock()
	defer stateMu.Unlock()

	if c.gameID != "" {
		return
	}

	removeWaitingLocked(c)
	startBotGameLocked(c, level)
}

func handleMove(c *client, raw json.RawMessage) {
	var payload movePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		send(c, "game:error", map[string]string{"message": "Malformed move."})
		return
	}

	stateMu.Lock()
	g := games[c.gameID]
	stateMu.Unlock()
	if g == nil {
		send(c, "game:error", map[string]string{"message": "No active game."})
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.board.Position().Turn() != c.color {
		send(c, "game:error", map[string]string{"message": "It is not your turn."})
		return
	}

	moveText := coordinateMove(payload)
	// 합법 수 검증은 모두 서버에서 수행한다. 브라우저는 좌표만 보내고 chess 라이브러리가 판정한다.
	move, err := chess.UCINotation{}.Decode(g.board.Position(), moveText)
	if err != nil {
		send(c, "game:error", map[string]string{"message": "Illegal move."})
		return
	}

	if err := g.board.Move(move); err != nil {
		send(c, "game:error", map[string]string{"message": "Illegal move."})
		return
	}

	g.moves = append(g.moves, moveText)
	recordMove()
	cacheSetGame(g)
	broadcast(g, "game:update", gameView(g, chess.NoColor, map[string]interface{}{"lastMove": moveText}))
	persistIfFinished(g)
	if g.aiColor != chess.NoColor && g.board.Outcome() == chess.NoOutcome && g.board.Position().Turn() == g.aiColor {
		// 사람 수 직후 AI 수를 동기적으로 처리해 클라이언트가 서버 기준 보드 갱신만 따라가게 한다.
		playAIMove(g)
	}
}

func handleResign(c *client) {
	stateMu.Lock()
	g := games[c.gameID]
	stateMu.Unlock()
	if g == nil {
		send(c, "game:error", map[string]string{"message": "No active game."})
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.board.Outcome() != chess.NoOutcome {
		return
	}

	g.board.Resign(c.color)
	cacheSetGame(g)
	broadcast(g, "game:update", gameView(g, chess.NoColor, map[string]interface{}{"resigned": colorToken(c.color)}))
	persistIfFinished(g)
}

func handleRematch(c *client) {
	stateMu.Lock()
	g := games[c.gameID]
	stateMu.Unlock()
	if g == nil {
		send(c, "game:error", map[string]string{"message": "No active game."})
		return
	}

	g.mu.Lock()
	if g.board.Outcome() == chess.NoOutcome {
		g.mu.Unlock()
		send(c, "game:error", map[string]string{"message": "Game is still in progress."})
		return
	}

	if g.aiColor != chess.NoColor {
		level := g.aiLevel
		g.mu.Unlock()
		stateMu.Lock()
		delete(games, g.id)
		cacheDeleteGame(g.id)
		stateMu.Unlock()
		startBotGameLocked(c, level)
		return
	}

	if g.rematch == nil {
		g.rematch = map[string]bool{}
	}
	g.rematch[c.id] = true
	whiteReady := g.white != nil && g.rematch[g.white.id]
	blackReady := g.black != nil && g.rematch[g.black.id]
	white := g.white
	black := g.black
	roomCode := g.roomCode
	g.mu.Unlock()

	if !whiteReady || !blackReady {
		send(c, "game:rematch_requested", map[string]interface{}{
			"gameId":       g.id,
			"rematchReady": false,
		})
		if opponent := rematchOpponent(g, c); opponent != nil {
			send(opponent, "game:rematch_requested", map[string]interface{}{
				"gameId":       g.id,
				"rematchReady": false,
			})
		}
		return
	}

	stateMu.Lock()
	if roomCode != "" && len(g.spectators) > 0 {
		if roomSpectators[roomCode] == nil {
			roomSpectators[roomCode] = map[string]*client{}
		}
		for id, spectator := range g.spectators {
			roomSpectators[roomCode][id] = spectator
		}
	}
	delete(games, g.id)
	cacheDeleteGame(g.id)
	if white != nil && black != nil {
		startGameWithRoomLocked(black, white, roomCode)
	}
	stateMu.Unlock()
}

func rematchOpponent(g *game, c *client) *client {
	if g == nil {
		return nil
	}
	if g.white == c {
		return g.black
	}
	if g.black == c {
		return g.white
	}
	return nil
}

func handleAnalysis(c *client) {
	stateMu.Lock()
	g := games[c.gameID]
	stateMu.Unlock()
	if g == nil {
		send(c, "game:error", map[string]string{"message": "No active game."})
		return
	}

	g.mu.Lock()
	// 분석은 복제한 보드에서 수행한다. Stockfish/heuristic 분석이 실제 게임 상태를 바꾸면 안 된다.
	board := g.board.Clone()
	level := g.aiLevel
	g.mu.Unlock()

	result := analyzePosition(board, level)
	send(c, "game:analysis", result)
}

func handleClose(c *client) {
	recordConnectionClosed()

	stateMu.Lock()
	removeWaitingLocked(c)
	removeRoomOwnerLocked(c)
	removeSpectatorLocked(c)

	var opponent *client
	if c.gameID != "" {
		if g := games[c.gameID]; g != nil {
			if g.white == c {
				opponent = g.black
			} else if g.black == c {
				opponent = g.white
			} else {
				delete(g.spectators, c.id)
			}
			if g.white == c || g.black == c {
				delete(games, g.id)
				cacheDeleteGame(g.id)
			}
		}
	}
	stateMu.Unlock()

	if opponent != nil && opponent.conn != nil {
		send(opponent, "game:opponent_disconnected", map[string]string{"gameId": c.gameID})
	}
	_ = c.conn.Close()
}

func gameView(g *game, color chess.Color, extra map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"gameId":   g.id,
		"fen":      g.board.FEN(),
		"turn":     colorToken(g.board.Position().Turn()),
		"moves":    g.moves,
		"status":   gameStatus(g.board),
		"outcome":  g.board.Outcome().String(),
		"method":   g.board.Method().String(),
		"winner":   winnerToken(g.board.Outcome()),
		"roomCode": g.roomCode,
	}
	if color == chess.White || color == chess.Black {
		payload["color"] = colorToken(color)
	}
	if g.aiColor != chess.NoColor {
		payload["mode"] = "bot"
		payload["aiColor"] = colorToken(g.aiColor)
		payload["aiLevel"] = g.aiLevel
		payload["aiEngine"] = g.aiEngine
	} else {
		payload["mode"] = "multiplayer"
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func gameStatus(board *chess.Game) string {
	if board.Method() == chess.Resignation {
		return "resignation"
	}
	switch board.Outcome() {
	case chess.NoOutcome:
		return "active"
	case chess.Draw:
		return "draw"
	default:
		return "checkmate"
	}
}

func winnerToken(outcome chess.Outcome) string {
	switch outcome {
	case chess.WhiteWon:
		return "w"
	case chess.BlackWon:
		return "b"
	default:
		return ""
	}
}

func colorToken(color chess.Color) string {
	if color == chess.White {
		return "w"
	}
	return "b"
}

func decodeAuthPayload(w http.ResponseWriter, r *http.Request) (authPayload, bool) {
	defer r.Body.Close()
	var payload authPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Malformed JSON body."})
		return payload, false
	}
	payload.Username = strings.ToLower(strings.TrimSpace(payload.Username))
	return payload, true
}

func validateAuthPayload(payload authPayload) error {
	if len(payload.Username) < 3 || len(payload.Username) > 24 {
		return errors.New("Username must be 3 to 24 characters.")
	}
	for _, char := range payload.Username {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return errors.New("Username can use letters, numbers, underscores, and hyphens.")
	}
	if len(payload.Password) < 8 {
		return errors.New("Password must be at least 8 characters.")
	}
	return nil
}

func createUser(ctx context.Context, payload authPayload) (userAccount, error) {
	hash, err := hashPassword(payload.Password)
	if err != nil {
		return userAccount{}, err
	}
	account := userAccount{
		ID:           secureToken(12),
		Username:     payload.Username,
		PasswordHash: hash,
		IsAdmin:      isAdminUsername(payload.Username),
		CreatedAt:    time.Now().UTC(),
	}
	if dbPool != nil {
		_, err := dbPool.Exec(ctx, `
			insert into users (id, username, password_hash, is_admin, created_at)
			values ($1, $2, $3, $4, $5)
		`, account.ID, account.Username, account.PasswordHash, account.IsAdmin, account.CreatedAt)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				return userAccount{}, pgx.ErrNoRows
			}
			return userAccount{}, err
		}
		return account, nil
	}

	authMu.Lock()
	defer authMu.Unlock()
	if _, exists := memoryUsers[account.Username]; exists {
		return userAccount{}, pgx.ErrNoRows
	}
	memoryUsers[account.Username] = account
	return account, nil
}

func authenticateUser(ctx context.Context, payload authPayload) (userAccount, error) {
	username := strings.ToLower(strings.TrimSpace(payload.Username))
	var account userAccount
	var err error
	if dbPool != nil {
		err = dbPool.QueryRow(ctx, `
			select id, username, password_hash, is_admin, created_at
			from users
			where username = $1
		`, username).Scan(&account.ID, &account.Username, &account.PasswordHash, &account.IsAdmin, &account.CreatedAt)
	} else {
		authMu.Lock()
		account, err = memoryUsers[username], nil
		_, ok := memoryUsers[username]
		authMu.Unlock()
		if !ok {
			err = pgx.ErrNoRows
		}
	}
	if err != nil {
		return userAccount{}, errAuthFailed
	}
	if !verifyPassword(payload.Password, account.PasswordHash) {
		return userAccount{}, errAuthFailed
	}
	if isAdminUsername(account.Username) && !account.IsAdmin {
		account.IsAdmin = true
	}
	return account, nil
}

func authView(account userAccount) map[string]interface{} {
	return map[string]interface{}{
		"id":        account.ID,
		"username":  account.Username,
		"isAdmin":   account.IsAdmin,
		"createdAt": account.CreatedAt.Format(time.RFC3339),
	}
}

func isAdminUsername(username string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	for _, entry := range strings.Split(os.Getenv("ADMIN_USERS"), ",") {
		if strings.ToLower(strings.TrimSpace(entry)) == username && username != "" {
			return true
		}
	}
	return false
}

func startSession(w http.ResponseWriter, r *http.Request, account userAccount) {
	token := secureToken(32)
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	record := sessionRecord{Account: account, ExpiresAt: expiresAt}
	authMu.Lock()
	sessions[token] = record
	authMu.Unlock()
	// Redis 세션은 재시작 또는 다중 인스턴스 상황에서 로그인 상태 복구를 돕는다.
	// 메모리 세션은 로컬 실행을 위한 fallback이다.
	cacheSetSession(token, record)
	http.SetCookie(w, &http.Cookie{
		Name:     "chess_session",
		Value:    token,
		Path:     "/",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
	setCSRFCookie(w, r)
}

func currentUser(r *http.Request) (userAccount, bool) {
	cookie, err := r.Cookie("chess_session")
	if err != nil || cookie.Value == "" {
		return userAccount{}, false
	}
	authMu.Lock()
	record, ok := sessions[cookie.Value]
	if ok && time.Now().Before(record.ExpiresAt) {
		authMu.Unlock()
		return record.Account, true
	}
	if ok {
		delete(sessions, cookie.Value)
	}
	authMu.Unlock()

	// 메모리에 세션이 없으면 Redis를 조회한다. Redis 조회 성공 시 다시 메모리에 올려 이후 요청을 빠르게 처리한다.
	record, ok = cacheGetSession(cookie.Value)
	if !ok || time.Now().After(record.ExpiresAt) {
		return userAccount{}, false
	}
	authMu.Lock()
	sessions[cookie.Value] = record
	authMu.Unlock()
	return record.Account, true
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chess_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chess_csrf",
		Value:    secureToken(24),
		Path:     "/",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func clearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chess_csrf",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func validateCSRF(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	// Double-submit CSRF 방식이다. 읽을 수 있는 쿠키 값과 커스텀 헤더 값이 같아야 통과한다.
	cookie, err := r.Cookie("chess_csrf")
	if err != nil || cookie.Value == "" {
		return false
	}
	header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) == 1
}

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func checkWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" {
		return false
	}

	requestHost := forwardedHost(r)
	if strings.EqualFold(originURL.Host, requestHost) {
		return true
	}

	if isLoopbackHost(originURL.Hostname()) && isLoopbackHost(hostnameOnly(requestHost)) {
		return true
	}

	for _, allowed := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		allowedURL, err := url.Parse(allowed)
		if err == nil && allowedURL.Host != "" && strings.EqualFold(originURL.Host, allowedURL.Host) {
			return true
		}
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}

func forwardedHost(r *http.Request) string {
	if host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); host != "" {
		return strings.Split(host, ",")[0]
	}
	return r.Host
}

func hostnameOnly(host string) string {
	name, _, err := net.SplitHostPort(host)
	if err == nil {
		return name
	}
	return host
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func allowAuthRequest(r *http.Request) bool {
	key := clientIP(r)
	now := time.Now()
	rateMu.Lock()
	defer rateMu.Unlock()

	bucket := authBuckets[key]
	if bucket == nil || now.After(bucket.reset) {
		authBuckets[key] = &rateBucket{tokens: 9, reset: now.Add(time.Minute)}
		return true
	}
	if bucket.tokens <= 0 {
		return false
	}
	bucket.tokens--
	return true
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := crand.Read(salt); err != nil {
		return "", err
	}
	const iterations = 120000
	key := pbkdf2SHA256([]byte(password), salt, iterations, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(password string, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password []byte, salt []byte, iterations int, keyLen int) []byte {
	hashLen := 32
	numBlocks := (keyLen + hashLen - 1) / hashLen
	out := make([]byte, 0, numBlocks*hashLen)
	for block := 1; block <= numBlocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func recentGames(ctx context.Context, userID string, limit int) ([]gameRecord, error) {
	if dbPool == nil {
		historyMu.Lock()
		defer historyMu.Unlock()
		records := make([]gameRecord, 0, min(limit, len(memoryGameHistory)))
		for i := len(memoryGameHistory) - 1; i >= 0 && len(records) < limit; i-- {
			record := memoryGameHistory[i]
			if record.WhiteUserID == userID || record.BlackUserID == userID {
				records = append(records, record)
			}
		}
		return records, nil
	}

	rows, err := dbPool.Query(ctx, `
		select id, mode, coalesce(room_code, ''), moves, final_fen, outcome, method,
			coalesce(winner, ''), coalesce(ai_level, ''), coalesce(ai_engine, ''),
			coalesce(white_user_id, ''), coalesce(white_username, ''),
			coalesce(black_user_id, ''), coalesce(black_username, ''),
			started_at, ended_at
		from games
		where white_user_id = $1 or black_user_id = $1
		order by ended_at desc
		limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []gameRecord{}
	for rows.Next() {
		var record gameRecord
		if err := rows.Scan(
			&record.ID,
			&record.Mode,
			&record.RoomCode,
			&record.Moves,
			&record.FinalFEN,
			&record.Outcome,
			&record.Method,
			&record.Winner,
			&record.AILevel,
			&record.AIEngine,
			&record.WhiteUserID,
			&record.WhiteUsername,
			&record.BlackUserID,
			&record.BlackUsername,
			&record.StartedAt,
			&record.EndedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func statsForUser(ctx context.Context, userID string) (gameStats, error) {
	records, err := allGamesForUser(ctx, userID)
	if err != nil {
		return gameStats{}, err
	}
	stats := gameStats{}
	for _, record := range records {
		stats.Total++
		if len(record.Moves) > stats.LongestGame {
			stats.LongestGame = len(record.Moves)
		}
		if record.EndedAt.Format(time.RFC3339) > stats.LastPlayed {
			stats.LastPlayed = record.EndedAt.Format(time.RFC3339)
		}
		switch userResult(record, userID) {
		case "win":
			stats.Wins++
		case "draw":
			stats.Draws++
		case "loss":
			stats.Losses++
		}
	}
	if stats.Total > 0 {
		stats.WinRate = int(float64(stats.Wins) / float64(stats.Total) * 100)
	}
	return stats, nil
}

func allGamesForUser(ctx context.Context, userID string) ([]gameRecord, error) {
	if dbPool == nil {
		historyMu.Lock()
		defer historyMu.Unlock()
		records := []gameRecord{}
		for _, record := range memoryGameHistory {
			if record.WhiteUserID == userID || record.BlackUserID == userID {
				records = append(records, record)
			}
		}
		return records, nil
	}

	rows, err := dbPool.Query(ctx, `
		select id, mode, coalesce(room_code, ''), moves, final_fen, outcome, method,
			coalesce(winner, ''), coalesce(ai_level, ''), coalesce(ai_engine, ''),
			coalesce(white_user_id, ''), coalesce(white_username, ''),
			coalesce(black_user_id, ''), coalesce(black_username, ''),
			started_at, ended_at
		from games
		where white_user_id = $1 or black_user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []gameRecord{}
	for rows.Next() {
		var record gameRecord
		if err := rows.Scan(
			&record.ID,
			&record.Mode,
			&record.RoomCode,
			&record.Moves,
			&record.FinalFEN,
			&record.Outcome,
			&record.Method,
			&record.Winner,
			&record.AILevel,
			&record.AIEngine,
			&record.WhiteUserID,
			&record.WhiteUsername,
			&record.BlackUserID,
			&record.BlackUsername,
			&record.StartedAt,
			&record.EndedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func gameForUser(ctx context.Context, userID string, gameID string) (gameRecord, error) {
	if dbPool == nil {
		historyMu.Lock()
		defer historyMu.Unlock()
		for _, record := range memoryGameHistory {
			if record.ID == gameID && (record.WhiteUserID == userID || record.BlackUserID == userID) {
				return record, nil
			}
		}
		return gameRecord{}, pgx.ErrNoRows
	}

	var record gameRecord
	err := dbPool.QueryRow(ctx, `
		select id, mode, coalesce(room_code, ''), moves, final_fen, outcome, method,
			coalesce(winner, ''), coalesce(ai_level, ''), coalesce(ai_engine, ''),
			coalesce(white_user_id, ''), coalesce(white_username, ''),
			coalesce(black_user_id, ''), coalesce(black_username, ''),
			started_at, ended_at
		from games
		where id = $1 and (white_user_id = $2 or black_user_id = $2)
	`, gameID, userID).Scan(
		&record.ID,
		&record.Mode,
		&record.RoomCode,
		&record.Moves,
		&record.FinalFEN,
		&record.Outcome,
		&record.Method,
		&record.Winner,
		&record.AILevel,
		&record.AIEngine,
		&record.WhiteUserID,
		&record.WhiteUsername,
		&record.BlackUserID,
		&record.BlackUsername,
		&record.StartedAt,
		&record.EndedAt,
	)
	if err != nil {
		return gameRecord{}, err
	}
	return record, nil
}

func buildPGNExport(record gameRecord) (string, error) {
	board := chess.NewGame(chess.TagPairs([]*chess.TagPair{
		{Key: "Event", Value: "Linux Chess"},
		{Key: "Site", Value: "Local"},
		{Key: "Date", Value: pgnDate(record.EndedAt)},
		{Key: "Round", Value: "-"},
		{Key: "White", Value: pgnPlayerName(record.WhiteUsername, "White")},
		{Key: "Black", Value: pgnPlayerName(record.BlackUsername, "AI")},
		{Key: "Result", Value: pgnResult(record.Outcome)},
		{Key: "Termination", Value: pgnTermination(record.Method)},
		{Key: "Mode", Value: pgnMode(record.Mode)},
	}))

	notation := chess.AlgebraicNotation{}
	moveTexts := make([]string, 0, len(record.Moves))
	for _, moveText := range record.Moves {
		move, err := chess.UCINotation{}.Decode(board.Position(), moveText)
		if err != nil {
			return "", err
		}
		if err := board.Move(move); err != nil {
			return "", err
		}
		moveTexts = append(moveTexts, notation.Encode(board.Positions()[len(board.Moves())-1], move))
	}

	var out strings.Builder
	for _, tag := range board.TagPairs() {
		out.WriteString(fmt.Sprintf("[%s \"%s\"]\n", tag.Key, tag.Value))
	}
	out.WriteString("\n")
	tokens := make([]string, 0, len(moveTexts))
	for i, moveText := range moveTexts {
		if i%2 == 0 {
			tokens = append(tokens, fmt.Sprintf("%d. %s", (i/2)+1, moveText))
			continue
		}
		tokens = append(tokens, moveText)
	}
	out.WriteString(strings.Join(tokens, " "))
	out.WriteString(" ")
	out.WriteString(pgnResult(record.Outcome))
	return out.String(), nil
}

func pgnPlayerName(value string, fallback string) string {
	name := strings.TrimSpace(value)
	if name != "" {
		return name
	}
	return fallback
}

func pgnResult(outcome string) string {
	outcome = strings.TrimSpace(outcome)
	if outcome == "" {
		return "*"
	}
	return outcome
}

func pgnTermination(method string) string {
	method = strings.TrimSpace(method)
	if method == "" || method == "NoMethod" {
		return "Unspecified"
	}
	return method
}

func pgnMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "multiplayer"
	}
	return mode
}

func pgnDate(value time.Time) string {
	if value.IsZero() {
		return "????.??.??"
	}
	return value.UTC().Format("2006.01.02")
}

func userResult(record gameRecord, userID string) string {
	if record.Outcome == "1/2-1/2" || strings.EqualFold(record.Outcome, "Draw") {
		return "draw"
	}
	if record.Winner == "w" && record.WhiteUserID == userID {
		return "win"
	}
	if record.Winner == "b" && record.BlackUserID == userID {
		return "win"
	}
	if record.Winner == "w" || record.Winner == "b" {
		return "loss"
	}
	return ""
}

func coordinateMove(payload movePayload) string {
	move := strings.ToLower(payload.From + payload.To)
	if isPromotionMove(payload) {
		move += strings.ToLower(payload.Promotion)
	}
	return move
}

func playAIMove(g *game) {
	move, engine := chooseAIMove(g.board, g.aiLevel)
	if move == nil {
		return
	}
	g.aiEngine = engine

	moveText := move.String()
	if err := g.board.Move(move); err != nil {
		logJSON("error", "ai_move_failed", map[string]interface{}{"game_id": g.id, "move": moveText, "error": err.Error()})
		return
	}

	g.moves = append(g.moves, moveText)
	recordMove()
	cacheSetGame(g)
	broadcast(g, "game:update", gameView(g, chess.NoColor, map[string]interface{}{"lastMove": moveText, "movedBy": "ai"}))
	persistIfFinished(g)
}

func chooseAIMove(board *chess.Game, level string) (*chess.Move, string) {
	if move := stockfishMove(board, level); move != nil {
		return move, "stockfish"
	}
	return chooseHeuristicMove(board), "heuristic"
}

func chooseHeuristicMove(board *chess.Game) *chess.Move {
	moves := board.ValidMoves()
	if len(moves) == 0 {
		return nil
	}

	bestScore := -1_000_000
	var best []*chess.Move
	for _, move := range moves {
		score := scoreMove(board, move)
		if score > bestScore {
			bestScore = score
			best = []*chess.Move{move}
			continue
		}
		if score == bestScore {
			best = append(best, move)
		}
	}
	return best[mrand.Intn(len(best))]
}

func stockfishMove(board *chess.Game, level string) *chess.Move {
	path := stockfishPath()
	if path == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	movetime := aiMoveTime(level)
	commands := []string{
		"uci",
		"isready",
		"ucinewgame",
		"position fen " + board.FEN(),
		"go movetime " + movetime,
	}
	for _, command := range commands {
		if _, err := stdin.Write([]byte(command + "\n")); err != nil {
			return nil
		}
	}

	scanner := bufio.NewScanner(stdout)
	deadline := time.After(2500 * time.Millisecond)
	bestMove := ""
	for {
		select {
		case <-deadline:
			return nil
		default:
			if !scanner.Scan() {
				return nil
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "bestmove ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					bestMove = fields[1]
				}
				goto decode
			}
		}
	}

decode:
	if bestMove == "" || bestMove == "(none)" {
		return nil
	}
	move, err := chess.UCINotation{}.Decode(board.Position(), bestMove)
	if err != nil {
		return nil
	}
	return move
}

func analyzePosition(board *chess.Game, level string) map[string]interface{} {
	bestMove, score, engine := stockfishBestMoveAndScore(board, level)
	if bestMove == "" {
		move, fallback := chooseAIMove(board, level)
		engine = fallback
		if move != nil {
			bestMove = move.String()
		}
	}
	return map[string]interface{}{
		"engine":   engine,
		"bestMove": bestMove,
		"score":    score,
		"fen":      board.FEN(),
	}
}

func stockfishBestMoveAndScore(board *chess.Game, level string) (string, string, string) {
	path := stockfishPath()
	if path == "" {
		return "", "", "heuristic"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", "heuristic"
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", "heuristic"
	}
	if err := cmd.Start(); err != nil {
		return "", "", "heuristic"
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	for _, command := range []string{
		"uci",
		"isready",
		"position fen " + board.FEN(),
		"go movetime " + aiMoveTime(level),
	} {
		if _, err := stdin.Write([]byte(command + "\n")); err != nil {
			return "", "", "heuristic"
		}
	}

	scanner := bufio.NewScanner(stdout)
	deadline := time.After(3500 * time.Millisecond)
	score := ""
	for {
		select {
		case <-deadline:
			return "", score, "stockfish"
		default:
			if !scanner.Scan() {
				return "", score, "stockfish"
			}
			line := scanner.Text()
			if strings.Contains(line, " score ") {
				score = parseScore(line)
			}
			if strings.HasPrefix(line, "bestmove ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					return fields[1], score, "stockfish"
				}
				return "", score, "stockfish"
			}
		}
	}
}

func parseScore(line string) string {
	fields := strings.Fields(line)
	for i := 0; i+2 < len(fields); i++ {
		if fields[i] == "score" {
			return fields[i+1] + " " + fields[i+2]
		}
	}
	return ""
}

func stockfishPath() string {
	if path := os.Getenv("STOCKFISH_PATH"); path != "" {
		return path
	}
	if path := findBundledStockfish(); path != "" {
		return path
	}
	path, err := exec.LookPath("stockfish")
	if err != nil {
		return ""
	}
	return path
}

func findBundledStockfish() string {
	const relativePath = "tools/stockfish/stockfish/stockfish-ubuntu-x86-64-avx2"
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, relativePath)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func aiEngineName() string {
	if stockfishPath() != "" {
		return "stockfish"
	}
	return "heuristic"
}

func normalizeAILevel(level string) string {
	switch strings.ToLower(level) {
	case "easy", "medium", "hard":
		return strings.ToLower(level)
	default:
		return "medium"
	}
}

func aiMoveTime(level string) string {
	switch normalizeAILevel(level) {
	case "easy":
		return "80"
	case "hard":
		return "800"
	default:
		return "250"
	}
}

func scoreMove(board *chess.Game, move *chess.Move) int {
	score := 0
	if move.HasTag(chess.Capture) {
		score += 100
	}
	if move.HasTag(chess.Check) {
		score += 40
	}
	if move.Promo() == chess.Queen {
		score += 80
	}
	if isCenterSquare(move.S2().String()) {
		score += 12
	}

	clone := board.Clone()
	if err := clone.Move(move); err == nil && clone.Method() == chess.Checkmate {
		score += 10_000
	}
	return score
}

func isCenterSquare(square string) bool {
	switch square {
	case "d4", "e4", "d5", "e5":
		return true
	default:
		return false
	}
}

func isPromotionMove(payload movePayload) bool {
	if payload.Promotion == "" || len(payload.From) != 2 || len(payload.To) != 2 {
		return false
	}
	fromRank := payload.From[1]
	toRank := payload.To[1]
	return (fromRank == '7' && toRank == '8') || (fromRank == '2' && toRank == '1')
}

func broadcast(g *game, messageType string, payload interface{}) {
	if g.white != nil {
		send(g.white, messageType, payload)
	}
	if g.black != nil {
		send(g.black, messageType, payload)
	}
	for _, spectator := range g.spectators {
		send(spectator, messageType, payload)
	}
}

func send(c *client, messageType string, payload interface{}) {
	if c == nil || c.conn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.conn.WriteJSON(outboundMessage{Type: messageType, Payload: payload}); err != nil {
		logJSON("error", "client_write_failed", map[string]interface{}{"client_id": c.id, "message_type": messageType, "error": err.Error()})
	}
}

func recordConnectionOpened() {
	statsMu.Lock()
	defer statsMu.Unlock()
	activeConnections++
	totalConnections++
}

func recordConnectionClosed() {
	statsMu.Lock()
	defer statsMu.Unlock()
	if activeConnections > 0 {
		activeConnections--
	}
	disconnects++
}

func recordMove() {
	statsMu.Lock()
	defer statsMu.Unlock()
	totalMoves++
}

func recordCompletedGame(g *game) {
	if g.counted {
		return
	}
	g.counted = true
	statsMu.Lock()
	defer statsMu.Unlock()
	completedGames++
}

func removeWaitingLocked(c *client) {
	for i, entry := range waiting {
		if entry == c {
			waiting = append(waiting[:i], waiting[i+1:]...)
			return
		}
	}
}

func removeRoomOwnerLocked(c *client) {
	for code, owner := range rooms {
		if owner == c {
			delete(rooms, code)
			notifyRoomClosedLocked(code)
		}
	}
}

func removeSpectatorLocked(c *client) {
	if c == nil || !c.spectating {
		return
	}
	if c.watchCode != "" {
		if spectators := roomSpectators[c.watchCode]; spectators != nil {
			delete(spectators, c.id)
			if len(spectators) == 0 {
				delete(roomSpectators, c.watchCode)
			}
		}
	}
	if c.gameID != "" {
		if g := games[c.gameID]; g != nil {
			delete(g.spectators, c.id)
		}
	}
}

func notifyRoomClosedLocked(code string) {
	spectators := roomSpectators[code]
	if len(spectators) == 0 {
		delete(roomSpectators, code)
		return
	}
	for id, spectator := range spectators {
		send(spectator, "room:closed", map[string]string{"code": code})
		spectator.spectating = false
		spectator.watchCode = ""
		delete(spectators, id)
	}
	delete(roomSpectators, code)
}

func uniqueRoomCodeLocked() string {
	for {
		code := strings.ToUpper(randomID(6))
		if rooms[code] == nil {
			return code
		}
	}
}

func randomID(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, length)
	for i := range out {
		out[i] = alphabet[mrand.Intn(len(alphabet))]
	}
	return string(out)
}

func secureToken(length int) string {
	bytes := make([]byte, length)
	if _, err := crand.Read(bytes); err != nil {
		return randomID(length * 2)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func initStore() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return
	}
	var lastErr error
	for attempt := 1; attempt <= 12; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			err = pool.Ping(ctx)
		}
		cancel()
		if err == nil {
			dbPool = pool
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			if err := ensureSchema(ctx); err != nil {
				logJSON("error", "database_schema_setup_failed", map[string]interface{}{"error": err.Error()})
			}
			cancel()
			return
		}
		if pool != nil {
			pool.Close()
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	logJSON("warn", "database_disabled", map[string]interface{}{"error": lastErr.Error()})
}

func initCache() {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		return
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		logJSON("warn", "redis_disabled", map[string]interface{}{"error": err.Error()})
		return
	}
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		logJSON("warn", "redis_disabled", map[string]interface{}{"error": err.Error()})
		return
	}
	cache = client
}

func closeStore() {
	if dbPool != nil {
		dbPool.Close()
	}
}

func closeCache() {
	if cache != nil {
		_ = cache.Close()
	}
}

func ensureSchema(ctx context.Context) error {
	_, err := dbPool.Exec(ctx, `
		create table if not exists users (
			id text primary key,
			username text not null unique,
			password_hash text not null,
			is_admin boolean not null default false,
			created_at timestamptz not null default now()
		);

		create table if not exists games (
			id text primary key,
			mode text not null,
			room_code text,
			moves text[] not null,
			final_fen text not null,
			outcome text not null,
			method text not null,
			winner text,
			ai_level text,
			ai_engine text,
			white_user_id text,
			white_username text,
			black_user_id text,
			black_username text,
			started_at timestamptz not null,
			ended_at timestamptz not null default now()
		);

		alter table games add column if not exists white_user_id text;
		alter table games add column if not exists white_username text;
		alter table games add column if not exists black_user_id text;
		alter table games add column if not exists black_username text;
		alter table users add column if not exists is_admin boolean not null default false;
	`)
	return err
}

func persistIfFinished(g *game) {
	if g.board.Outcome() == chess.NoOutcome {
		return
	}
	cacheDeleteGame(g.id)
	recordCompletedGame(g)
	if g.persisted {
		return
	}
	g.persisted = true
	record := buildGameRecord(g)
	storeMemoryGame(record)
	if dbPool == nil {
		return
	}
	persistGameRecord(record)
}

func buildGameRecord(g *game) gameRecord {
	mode := "multiplayer"
	if g.aiColor != chess.NoColor {
		mode = "bot"
	}
	record := gameRecord{
		ID:        g.id,
		Mode:      mode,
		RoomCode:  g.roomCode,
		Moves:     append([]string(nil), g.moves...),
		FinalFEN:  g.board.FEN(),
		Outcome:   g.board.Outcome().String(),
		Method:    g.board.Method().String(),
		Winner:    winnerToken(g.board.Outcome()),
		AILevel:   g.aiLevel,
		AIEngine:  g.aiEngine,
		StartedAt: g.createdAt,
		EndedAt:   time.Now().UTC(),
	}
	if g.white != nil {
		record.WhiteUserID = g.white.userID
		record.WhiteUsername = g.white.username
	}
	if g.black != nil {
		record.BlackUserID = g.black.userID
		record.BlackUsername = g.black.username
	}
	return record
}

func storeMemoryGame(record gameRecord) {
	historyMu.Lock()
	defer historyMu.Unlock()
	memoryGameHistory = append(memoryGameHistory, record)
	if len(memoryGameHistory) > 200 {
		memoryGameHistory = memoryGameHistory[len(memoryGameHistory)-200:]
	}
}

func persistGameRecord(record gameRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := dbPool.Exec(ctx, `
		insert into games (
			id, mode, room_code, moves, final_fen, outcome, method, winner,
			ai_level, ai_engine, white_user_id, white_username, black_user_id, black_username, started_at, ended_at
		)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		on conflict (id) do nothing
	`,
		record.ID,
		record.Mode,
		nullable(record.RoomCode),
		record.Moves,
		record.FinalFEN,
		record.Outcome,
		record.Method,
		nullable(record.Winner),
		nullable(record.AILevel),
		nullable(record.AIEngine),
		nullable(record.WhiteUserID),
		nullable(record.WhiteUsername),
		nullable(record.BlackUserID),
		nullable(record.BlackUsername),
		record.StartedAt,
		record.EndedAt,
	)
	if err != nil {
		logJSON("error", "persist_game_failed", map[string]interface{}{"game_id": record.ID, "error": err.Error()})
	}
}

func currentRuntimeStatus() runtimeStatus {
	stateMu.Lock()
	waitingCount := len(waiting)
	roomCount := len(rooms)
	activeGames := len(games)
	stateMu.Unlock()

	statsMu.Lock()
	active := activeConnections
	total := totalConnections
	moves := totalMoves
	completed := completedGames
	disconnected := disconnects
	statsMu.Unlock()

	userCount, gameCount, recentUsers := adminDashboardData(context.Background())

	return runtimeStatus{
		ActiveConnections: active,
		TotalConnections:  total,
		WaitingPlayers:    waitingCount,
		OpenRooms:         roomCount,
		ActiveGames:       activeGames,
		CompletedGames:    completed,
		TotalMoves:        moves,
		Disconnects:       disconnected,
		DB:                dbPool != nil,
		Redis:             cache != nil,
		AIEngine:          aiEngineName(),
		ServerTime:        time.Now().UTC().Format(time.RFC3339),
		UserCount:         userCount,
		GameCount:         gameCount,
		RecentUsers:       recentUsers,
	}
}

func adminDashboardData(ctx context.Context) (int, int, []adminUserSummary) {
	if dbPool == nil {
		authMu.Lock()
		users := make([]adminUserSummary, 0, len(memoryUsers))
		for _, account := range memoryUsers {
			users = append(users, adminUserSummary{
				ID:        account.ID,
				Username:  account.Username,
				IsAdmin:   account.IsAdmin,
				CreatedAt: account.CreatedAt.Format(time.RFC3339),
			})
		}
		authMu.Unlock()

		historyMu.Lock()
		gameCount := len(memoryGameHistory)
		historyMu.Unlock()
		return len(users), gameCount, newestUsers(users, 5)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var userCount int
	var gameCount int
	if err := dbPool.QueryRow(ctx, `select count(*) from users`).Scan(&userCount); err != nil {
		logJSON("error", "admin_user_count_failed", map[string]interface{}{"error": err.Error()})
	}
	if err := dbPool.QueryRow(ctx, `select count(*) from games`).Scan(&gameCount); err != nil {
		logJSON("error", "admin_game_count_failed", map[string]interface{}{"error": err.Error()})
	}

	rows, err := dbPool.Query(ctx, `
		select id, username, is_admin, created_at
		from users
		order by created_at desc
		limit 5
	`)
	if err != nil {
		logJSON("error", "admin_recent_users_failed", map[string]interface{}{"error": err.Error()})
		return userCount, gameCount, nil
	}
	defer rows.Close()

	recent := []adminUserSummary{}
	for rows.Next() {
		var item adminUserSummary
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Username, &item.IsAdmin, &createdAt); err != nil {
			logJSON("error", "admin_recent_user_scan_failed", map[string]interface{}{"error": err.Error()})
			continue
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		recent = append(recent, item)
	}
	if err := rows.Err(); err != nil {
		logJSON("error", "admin_recent_users_rows_failed", map[string]interface{}{"error": err.Error()})
	}
	return userCount, gameCount, recent
}

func newestUsers(users []adminUserSummary, limit int) []adminUserSummary {
	for i := 0; i < len(users); i++ {
		for j := i + 1; j < len(users); j++ {
			if users[j].CreatedAt > users[i].CreatedAt {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
	if len(users) > limit {
		return users[:limit]
	}
	return users
}

func cacheSetRoom(code string, ownerID string) {
	if cache == nil || code == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	key := "chess:room:" + code
	payload := map[string]interface{}{"code": code, "ownerId": ownerID, "createdAt": time.Now().UTC().Format(time.RFC3339)}
	if err := cache.HSet(ctx, key, payload).Err(); err != nil {
		logJSON("error", "redis_room_cache_failed", map[string]interface{}{"room_code": code, "error": err.Error()})
		return
	}
	_ = cache.Expire(ctx, key, 15*time.Minute).Err()
}

func cacheSetSession(token string, record sessionRecord) {
	if cache == nil || token == "" {
		return
	}
	raw, err := json.Marshal(record)
	if err != nil {
		logJSON("error", "session_encode_failed", map[string]interface{}{"error": err.Error()})
		return
	}
	ttl := time.Until(record.ExpiresAt)
	if ttl <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := cache.Set(ctx, "chess:session:"+token, raw, ttl).Err(); err != nil {
		logJSON("error", "redis_session_cache_failed", map[string]interface{}{"error": err.Error()})
	}
}

func cacheGetSession(token string) (sessionRecord, bool) {
	if cache == nil || token == "" {
		return sessionRecord{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	raw, err := cache.Get(ctx, "chess:session:"+token).Bytes()
	if err != nil {
		return sessionRecord{}, false
	}
	var record sessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		logJSON("error", "session_decode_failed", map[string]interface{}{"error": err.Error()})
		return sessionRecord{}, false
	}
	return record, true
}

func cacheDeleteSession(token string) {
	if cache == nil || token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = cache.Del(ctx, "chess:session:"+token).Err()
}

func cacheDeleteRoom(code string) {
	if cache == nil || code == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = cache.Del(ctx, "chess:room:"+code).Err()
}

func cacheSetGame(g *game) {
	if cache == nil || g == nil {
		return
	}
	payload := map[string]interface{}{
		"id":        g.id,
		"roomCode":  g.roomCode,
		"fen":       g.board.FEN(),
		"turn":      colorToken(g.board.Position().Turn()),
		"moves":     len(g.moves),
		"status":    gameStatus(g.board),
		"aiLevel":   g.aiLevel,
		"aiEngine":  g.aiEngine,
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := cache.Set(ctx, "chess:game:"+g.id, raw, 30*time.Minute).Err(); err != nil {
		logJSON("error", "redis_game_cache_failed", map[string]interface{}{"game_id": g.id, "error": err.Error()})
	}
}

func cacheDeleteGame(gameID string) {
	if cache == nil || gameID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = cache.Del(ctx, "chess:game:"+gameID).Err()
}

func boolMetric(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func nullable(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
