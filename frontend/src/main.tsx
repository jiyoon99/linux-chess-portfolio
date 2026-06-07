import React, { useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Chess, Square } from "chess.js";
import { Activity, Bot, Circle, Crown, DoorOpen, Flag, History, LogIn, LogOut, Radio, RefreshCw, Search, Server, Swords, UserPlus } from "lucide-react";
import "./styles.css";

type ServerMessage = {
  type: string;
  payload?: {
    clientId?: string;
    userId?: string;
    username?: string;
    gameId?: string;
    color?: "w" | "b";
    fen?: string;
    turn?: "w" | "b";
    moves?: string[];
    status?: string;
    outcome?: string;
    method?: string;
    winner?: "w" | "b" | "";
    mode?: "multiplayer" | "bot";
    aiColor?: "w" | "b";
    aiLevel?: "easy" | "medium" | "hard";
    aiEngine?: string;
    code?: string;
    roomCode?: string;
    bestMove?: string;
    score?: string;
    engine?: string;
    message?: string;
  };
};

const files = ["a", "b", "c", "d", "e", "f", "g", "h"];
const promotionPieces = [
  { value: "q", label: "Queen", white: "♕", black: "♛" },
  { value: "r", label: "Rook", white: "♖", black: "♜" },
  { value: "b", label: "Bishop", white: "♗", black: "♝" },
  { value: "n", label: "Knight", white: "♘", black: "♞" }
];

type HealthState = {
  ok: boolean;
  aiEngine: string;
  db: boolean;
  redis: boolean;
} | null;

type AuthUser = {
  id: string;
  username: string;
  isAdmin?: boolean;
  createdAt: string;
};

type GameRecord = {
  id: string;
  mode: string;
  roomCode?: string;
  moves: string[];
  finalFen?: string;
  outcome: string;
  method: string;
  winner?: "w" | "b" | "";
  aiLevel?: string;
  aiEngine?: string;
  whiteUsername?: string;
  blackUsername?: string;
  startedAt?: string;
  endedAt: string;
};

type GameStats = {
  total: number;
  wins: number;
  draws: number;
  losses: number;
  winRate: number;
  lastPlayed?: string;
  longestGame: number;
};

type RuntimeStatus = {
  activeConnections: number;
  totalConnections: number;
  waitingPlayers: number;
  openRooms: number;
  activeGames: number;
  completedGames: number;
  totalMoves: number;
  disconnects: number;
  db: boolean;
  redis: boolean;
  aiEngine: string;
  serverTime: string;
  userCount: number;
  gameCount: number;
  recentUsers: {
    id: string;
    username: string;
    isAdmin: boolean;
    createdAt: string;
  }[];
};

function socketUrl() {
  const proto = window.location.protocol === "https:" ? "wss" : "ws";
  const host = import.meta.env.VITE_WS_HOST ?? window.location.host;
  return `${proto}://${host}/ws`;
}

function apiUrl(path: string) {
  const host = import.meta.env.VITE_API_HOST ?? window.location.host;
  return `${window.location.protocol}//${host}${path}`;
}

function csrfToken() {
  // 서버가 내려준 double-submit CSRF 쿠키를 logout 같은 인증 POST 요청 헤더에 실어 보낸다.
  return document.cookie
    .split("; ")
    .find((cookie) => cookie.startsWith("chess_csrf="))
    ?.split("=")[1] ?? "";
}

function App() {
  const socketRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const [status, setStatus] = useState("Connecting");
  const [clientId, setClientId] = useState("");
  const [color, setColor] = useState<"w" | "b" | undefined>();
  const [fen, setFen] = useState(new Chess().fen());
  const [turn, setTurn] = useState<"w" | "b">("w");
  const [moves, setMoves] = useState<string[]>([]);
  const [selected, setSelected] = useState<Square | null>(null);
  const [gameState, setGameState] = useState("idle");
  const [serverStatus, setServerStatus] = useState("active");
  const [outcome, setOutcome] = useState("*");
  const [method, setMethod] = useState("NoMethod");
  const [winner, setWinner] = useState<"w" | "b" | "">("");
  const [mode, setMode] = useState<"multiplayer" | "bot">("multiplayer");
  const [aiColor, setAIColor] = useState<"w" | "b" | undefined>();
  const [aiLevel, setAILevel] = useState<"easy" | "medium" | "hard">("medium");
  const [aiEngine, setAIEngine] = useState("heuristic");
  const [roomCode, setRoomCode] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [analysis, setAnalysis] = useState<{ bestMove: string; score: string; engine: string } | null>(null);
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [health, setHealth] = useState<HealthState>(null);
  const [authUser, setAuthUser] = useState<AuthUser | null>(null);
  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [authUsername, setAuthUsername] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [authError, setAuthError] = useState("");
  const [authLoading, setAuthLoading] = useState(false);
  const [recentGames, setRecentGames] = useState<GameRecord[]>([]);
  const [gameStats, setGameStats] = useState<GameStats | null>(null);
  const [runtimeStatus, setRuntimeStatus] = useState<RuntimeStatus | null>(null);
  const [recentLoading, setRecentLoading] = useState(false);
  const [selectedGame, setSelectedGame] = useState<GameRecord | null>(null);
  const [replayPly, setReplayPly] = useState(0);
  const [pendingPromotion, setPendingPromotion] = useState<{ from: Square; to: Square } | null>(null);

  const chess = useMemo(() => new Chess(fen), [fen]);
  const orientation = color === "b" ? [...files].reverse() : files;
  const ranks = color === "b" ? [1, 2, 3, 4, 5, 6, 7, 8] : [8, 7, 6, 5, 4, 3, 2, 1];
  const moveBook = useMemo(() => buildMoveBook(moves), [moves]);
  const captured = useMemo(() => buildCapturedPieces(moves, color), [moves, color]);
  const lastMove = moves[moves.length - 1];
  const checkedKingSquare = useMemo(() => {
    if (!chess.isCheck()) return "";
    return findKingSquare(chess, turn);
  }, [chess, turn]);
  const resultBanner = resultBannerText(serverStatus, method, winner, chess.isCheck());
  const selectedTargets = useMemo(() => {
    if (!selected) return new Set<string>();
    return new Set(
      chess
        .moves({ square: selected, verbose: true })
        .map((move) => move.to)
    );
  }, [chess, selected]);
  const replayState = useMemo(() => buildReplayState(selectedGame, replayPly), [selectedGame, replayPly]);
  const replayMoveRows = useMemo(() => (selectedGame ? buildMoveBook(selectedGame.moves) : []), [selectedGame]);

  useEffect(() => {
    // 첫 화면에서 서비스 상태와 기존 로그인 세션을 확인한 뒤 WebSocket을 연결한다.
    fetch(apiUrl("/health"))
      .then((response) => (response.ok ? response.json() : Promise.reject(new Error("health check failed"))))
      .then((payload: HealthState) => setHealth(payload))
      .catch(() => setHealth({ ok: false, aiEngine: "unknown", db: false, redis: false }));

    fetch(apiUrl("/auth/me"), { credentials: "include" })
      .then((response) => (response.ok ? response.json() : null))
      .then((payload: AuthUser | null) => setAuthUser(payload))
      .catch(() => setAuthUser(null));

    let reconnecting = false;
    const socket = new WebSocket(socketUrl());
    socketRef.current = socket;

    socket.addEventListener("open", () => {
      setConnected(true);
      setStatus("Connected");
    });

    socket.addEventListener("close", () => {
      if (reconnecting) return;
      setConnected(false);
      setStatus("Disconnected");
      setGameState("offline");
    });

    socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data) as ServerMessage;
      const payload = message.payload ?? {};

      // 서버가 보낸 권위 있는 상태만 UI에 반영한다. 클라이언트는 보드를 낙관적으로 갱신하지 않는다.
      if (message.type === "session:ready") {
        setClientId(payload.clientId ?? "");
        if (payload.username && !authUser) {
          setAuthUser({ id: payload.userId ?? payload.clientId ?? "", username: payload.username, createdAt: "", isAdmin: false });
        }
      }
      if (message.type === "matchmaking:waiting") {
        setGameState("waiting");
        setStatus("Waiting for opponent");
      }
      if (message.type === "room:created" || message.type === "room:waiting") {
        setRoomCode(payload.code ?? "");
        setGameState("waiting");
        setStatus(`Room ${payload.code ?? ""} waiting`);
      }
      if (message.type === "game:start") {
        setGameState("playing");
        setStatus("Game started");
        setRoomCode(payload.roomCode ?? "");
        setAnalysis(null);
        setColor(payload.color);
        setFen(payload.fen ?? new Chess().fen());
        setTurn(payload.turn ?? "w");
        setMoves(payload.moves ?? []);
        applyServerState(payload);
      }
      if (message.type === "game:update") {
        setFen(payload.fen ?? new Chess().fen());
        setTurn(payload.turn ?? "w");
        setMoves(payload.moves ?? []);
        applyServerState(payload);
        setStatus(statusText(payload.status, payload.method, payload.winner));
        if (payload.status && payload.status !== "active") setGameState("ended");
        setSelected(null);
        setPendingPromotion(null);
      }
      if (message.type === "game:error") {
        setStatus(payload.message ?? "Move rejected");
        setAnalysisLoading(false);
      }
      if (message.type === "game:analysis") {
        setAnalysisLoading(false);
        setAnalysis({
          bestMove: payload.bestMove ?? "",
          score: payload.score ?? "",
          engine: payload.engine ?? "heuristic"
        });
      }
      if (message.type === "game:opponent_disconnected") {
        setStatus("Opponent disconnected");
        setGameState("ended");
      }
      if (message.type === "game:rematch_requested") {
        setStatus("Rematch requested");
      }
    });

    return () => {
      reconnecting = true;
      socket.close();
    };
  }, [authUser?.id]);

  useEffect(() => {
    if (!authUser) {
      setRecentGames([]);
      setGameStats(null);
      setRuntimeStatus(null);
      return;
    }
    loadGameDashboard();
  }, [authUser?.id, gameState]);

  useEffect(() => {
    if (selectedGame) {
      setReplayPly(selectedGame.moves.length);
    }
  }, [selectedGame?.id]);

  function joinQueue() {
    socketRef.current?.send(JSON.stringify({ type: "matchmaking:join" }));
  }

  function joinBotGame() {
    socketRef.current?.send(JSON.stringify({ type: "bot:join", payload: { level: aiLevel } }));
  }

  function createRoom() {
    socketRef.current?.send(JSON.stringify({ type: "room:create" }));
  }

  function joinRoom() {
    const code = joinCode.trim().toUpperCase();
    if (!code) return;
    socketRef.current?.send(JSON.stringify({ type: "room:join", payload: { code } }));
  }

  function applyServerState(payload: NonNullable<ServerMessage["payload"]>) {
    setServerStatus(payload.status ?? "active");
    setOutcome(payload.outcome ?? "*");
    setMethod(payload.method ?? "NoMethod");
    setWinner(payload.winner ?? "");
    setMode(payload.mode ?? "multiplayer");
    setAIColor(payload.aiColor);
    if (payload.aiLevel) setAILevel(payload.aiLevel);
    setAIEngine(payload.aiEngine ?? "heuristic");
  }

  function selectSquare(square: Square) {
    if (gameState !== "playing" || turn !== color) return;
    const piece = chess.get(square);

    // chess.js는 UI 힌트용으로만 사용한다. 최종 합법 수 판정은 백엔드가 다시 수행한다.
    if (!selected) {
      if (piece?.color === color) setSelected(square);
      return;
    }

    if (selected === square) {
      setSelected(null);
      return;
    }

    if (needsPromotion(chess, selected, square)) {
      setPendingPromotion({ from: selected, to: square });
      return;
    }

    sendMove(selected, square, "q");
  }

  function sendMove(from: Square, to: Square, promotion: string) {
    socketRef.current?.send(
      JSON.stringify({
        type: "game:move",
        payload: { from, to, promotion }
      })
    );
  }

  function resign() {
    socketRef.current?.send(JSON.stringify({ type: "game:resign" }));
  }

  function requestAnalysis() {
    setAnalysis(null);
    setAnalysisLoading(true);
    socketRef.current?.send(JSON.stringify({ type: "game:analysis" }));
  }

  function requestRematch() {
    socketRef.current?.send(JSON.stringify({ type: "game:rematch" }));
    setStatus(mode === "bot" ? "Starting new game" : "Rematch requested");
  }

  function resetSession() {
    window.location.reload();
  }

  async function submitAuth(event: React.FormEvent) {
    event.preventDefault();
    setAuthError("");
    setAuthLoading(true);
    try {
      const response = await fetch(apiUrl(authMode === "login" ? "/auth/login" : "/auth/register"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username: authUsername, password: authPassword })
      });
      const payload = await response.json();
      if (!response.ok) {
        setAuthError(payload.error ?? "Authentication failed.");
        return;
      }
      setAuthUser(payload);
      setAuthPassword("");
      setAuthUsername("");
      loadGameDashboard();
    } catch {
      setAuthError("Authentication service is unavailable.");
    } finally {
      setAuthLoading(false);
    }
  }

  async function logout() {
    setAuthLoading(true);
    try {
      await fetch(apiUrl("/auth/logout"), {
        method: "POST",
        credentials: "include",
        headers: { "X-CSRF-Token": csrfToken() }
      });
      setAuthUser(null);
      setRecentGames([]);
      setGameStats(null);
      setRuntimeStatus(null);
    } finally {
      setAuthLoading(false);
    }
  }

  async function loadGameDashboard() {
    setRecentLoading(true);
    try {
      // 일반 사용자는 게임 기록/통계만, admin은 운영 상태까지 한 번에 갱신한다.
      const statusRequest = authUser?.isAdmin
        ? fetch(apiUrl("/admin/status"), { credentials: "include" })
        : Promise.resolve(null);
      const [recentResponse, statsResponse, statusResponse] = await Promise.all([
        fetch(apiUrl("/games/recent"), { credentials: "include" }),
        fetch(apiUrl("/games/stats"), { credentials: "include" }),
        statusRequest
      ]);
      if (recentResponse.ok) {
        const payload = await recentResponse.json();
        setRecentGames(payload.games ?? []);
      }
      if (statsResponse.ok) {
        setGameStats(await statsResponse.json());
      }
      if (statusResponse?.ok) {
        setRuntimeStatus(await statusResponse.json());
      } else {
        setRuntimeStatus(null);
      }
    } finally {
      setRecentLoading(false);
    }
  }

  async function openGameDetail(game: GameRecord) {
    setSelectedGame(game);
    setReplayPly(game.moves.length);
    try {
      const response = await fetch(apiUrl(`/games/detail?id=${encodeURIComponent(game.id)}`), { credentials: "include" });
      if (response.ok) {
        const detail = (await response.json()) as GameRecord;
        setSelectedGame(detail);
        setReplayPly(detail.moves.length);
      }
    } catch {
      setSelectedGame(game);
    }
  }

  return (
    <main className="shell">
      <section className="topbar">
        <div>
          <h1>Linux Chess</h1>
          <p>Sign in, play a match, review your history, and watch the service metrics.</p>
        </div>
        <div className="actions">
          <button onClick={joinQueue} disabled={!connected || gameState === "waiting" || gameState === "playing"}>
            <Swords size={18} />
            Find Match
          </button>
          <button onClick={joinBotGame} disabled={!connected || gameState === "waiting" || gameState === "playing"}>
            <Bot size={18} />
            Play AI
          </button>
          <button onClick={createRoom} disabled={!connected || gameState === "waiting" || gameState === "playing"}>
            <DoorOpen size={18} />
            Create Room
          </button>
          <button onClick={resetSession} aria-label="Reset session" title="Reset session">
            <RefreshCw size={18} />
          </button>
        </div>
      </section>

      <section className="workspace">
        <div className="boardPanel">
          <PlayerStrip
            side="top"
            colorLabel={opponentLabel(color, mode, aiColor)}
            active={turn !== color && gameState === "playing"}
            checked={chess.isCheck() && turn !== color}
            bot={mode === "bot"}
            captures={captured.byOpponent}
          />
          <div className="boardFrame">
            {resultBanner ? (
              <div className={`boardAlert ${serverStatus === "active" ? "checkAlert" : "mateAlert"}`}>
                <strong>{resultBanner.title}</strong>
                <span>{resultBanner.detail}</span>
              </div>
            ) : null}
            <BoardView
              board={chess}
              orientation={orientation}
              ranks={ranks}
              lastMove={lastMove}
              selected={selected}
              selectedTargets={selectedTargets}
              checkedKingSquare={checkedKingSquare}
              onSquareClick={selectSquare}
            />
          </div>
          <PlayerStrip
            side="bottom"
            colorLabel={color === "b" ? "Black" : "White"}
            active={turn === color && gameState === "playing"}
            checked={chess.isCheck() && turn === color}
            bot={false}
            captures={captured.byYou}
          />
        </div>

        <aside className="side">
          {gameState === "idle" || gameState === "waiting" ? (
            <div className="panel nextPanel">
              <div className="panelTitle">
                <Swords size={18} />
                Play
              </div>
              <p>{nextActionText(gameState, roomCode)}</p>
            </div>
          ) : null}

          <div className="panel matchPanel">
            <div className="panelTitle">
              <Crown size={18} />
              Match Control
            </div>
            <div className="metric">
              <span>Session</span>
              <strong>{clientId || "pending"}</strong>
            </div>
            {roomCode ? (
              <div className="metric">
                <span>Room</span>
                <strong>{roomCode}</strong>
              </div>
            ) : null}
            <div className="metric">
              <span>State</span>
              <strong>{gameStatusLabel(gameState)}</strong>
            </div>
            <div className="metric">
              <span>Mode</span>
              <strong>{mode === "bot" ? "AI opponent" : "Multiplayer"}</strong>
            </div>
            {mode === "bot" ? (
              <>
                <div className="metric">
                  <span>Engine</span>
                  <strong>{aiEngine}</strong>
                </div>
                <div className="metric">
                  <span>Level</span>
                  <strong>{aiLevel}</strong>
                </div>
              </>
            ) : (
              <div className="aiPicker">
                <span>AI level</span>
                <div>
                  {(["easy", "medium", "hard"] as const).map((level) => (
                    <button
                      className={aiLevel === level ? "selectedLevel" : ""}
                      key={level}
                      onClick={() => setAILevel(level)}
                      disabled={gameState === "playing"}
                    >
                      {level}
                    </button>
                  ))}
                </div>
              </div>
            )}
            <button className="resignButton" onClick={resign} disabled={gameState !== "playing"}>
              <Flag size={16} />
              Resign
            </button>
          </div>

          <div className="panel roomPanel">
            <div className="panelTitle">
              <DoorOpen size={18} />
              Join Room
            </div>
            <div className="roomJoin">
              <input
                value={joinCode}
                maxLength={6}
                onChange={(event) => setJoinCode(event.target.value.replace(/[^a-z0-9]/gi, "").toUpperCase())}
                placeholder="ABC123"
                disabled={!connected || gameState === "playing"}
              />
              <button onClick={joinRoom} disabled={!connected || !joinCode.trim() || gameState === "playing"}>
                Join
              </button>
            </div>
          </div>

          <div className="panel authPanel">
            <div className="panelTitle">
              {authUser ? <LogOut size={18} /> : authMode === "login" ? <LogIn size={18} /> : <UserPlus size={18} />}
              Account
            </div>
            {authUser ? (
              <div className="signedIn">
                <div>
                  <span>Signed in as</span>
                  <strong>{authUser.username}</strong>
                </div>
                <div className="profileStats">
                  <span>
                    <b>{gameStats?.total ?? 0}</b>
                    Games
                  </span>
                  <span>
                    <b>{gameStats?.wins ?? 0}-{gameStats?.draws ?? 0}-{gameStats?.losses ?? 0}</b>
                    W-D-L
                  </span>
                  <span>
                    <b>{gameStats?.winRate ?? 0}%</b>
                    Win rate
                  </span>
                </div>
                <button onClick={logout} disabled={authLoading}>
                  <LogOut size={16} />
                  Sign out
                </button>
              </div>
            ) : (
              <form className="authForm" onSubmit={submitAuth}>
                <div className="authTabs">
                  <button type="button" className={authMode === "login" ? "activeAuthTab" : ""} onClick={() => setAuthMode("login")}>
                    Sign in
                  </button>
                  <button type="button" className={authMode === "register" ? "activeAuthTab" : ""} onClick={() => setAuthMode("register")}>
                    Register
                  </button>
                </div>
                <input
                  value={authUsername}
                  onChange={(event) => setAuthUsername(event.target.value.toLowerCase())}
                  placeholder="username"
                  autoComplete="username"
                />
                <input
                  value={authPassword}
                  onChange={(event) => setAuthPassword(event.target.value)}
                  placeholder="password"
                  type="password"
                  autoComplete={authMode === "login" ? "current-password" : "new-password"}
                />
                {authError ? <p className="authError">{authError}</p> : null}
                <button type="submit" disabled={authLoading || !authUsername || !authPassword}>
                  {authLoading ? "Working..." : authMode === "login" ? "Sign in" : "Create account"}
                </button>
              </form>
            )}
          </div>

          {authUser ? (
            <div className="panel historyPanel">
              <div className="panelTitle">
                <History size={18} />
                Recent Games
              </div>
              <div className="historyList">
                {recentLoading ? (
                  <div className="emptyMoves">Loading games</div>
                ) : recentGames.length === 0 ? (
                  <div className="emptyMoves">No completed games yet</div>
                ) : (
                  recentGames.map((game) => (
                    <div className="historyItem" key={game.id}>
                      <div>
                        <strong>{gameResultText(game)}</strong>
                        <span>{gameOpponentText(game, authUser.username)}</span>
                      </div>
                      <div className="historyMeta">
                        <small>{game.moves.length} moves · {relativeDate(game.endedAt)}</small>
                        <button onClick={() => openGameDetail(game)}>Detail</button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          ) : null}

          <div className="panel statusPanel">
            <div className="panelTitle">
              <Activity size={18} />
              System
            </div>
            <div className={`statusRow ${connected ? "online" : "offline"}`}>
              <Radio size={18} />
              <span>{status}</span>
            </div>
            <div className="healthRow">
              <span>
                <Activity size={15} />
                API
              </span>
              <strong>{health ? (health.ok ? "online" : "offline") : "checking"}</strong>
              <span>AI</span>
              <strong>{health?.aiEngine ?? "checking"}</strong>
              <span>DB</span>
              <strong>{health ? (health.db ? "on" : "off") : "checking"}</strong>
              <span>Redis</span>
              <strong>{health ? (health.redis ? "on" : "off") : "checking"}</strong>
            </div>
            {authUser?.isAdmin && runtimeStatus ? (
              <div className="opsGrid">
                <div>
                  <span>WS</span>
                  <strong>{runtimeStatus.activeConnections}</strong>
                </div>
                <div>
                  <span>Games</span>
                  <strong>{runtimeStatus.activeGames}</strong>
                </div>
                <div>
                  <span>Rooms</span>
                  <strong>{runtimeStatus.openRooms}</strong>
                </div>
                <div>
                  <span>Moves</span>
                  <strong>{runtimeStatus.totalMoves}</strong>
                </div>
                <div>
                  <span>Users</span>
                  <strong>{runtimeStatus.userCount}</strong>
                </div>
                <div>
                  <span>Saved</span>
                  <strong>{runtimeStatus.gameCount}</strong>
                </div>
              </div>
            ) : null}
            {authUser?.isAdmin && runtimeStatus?.recentUsers?.length ? (
              <div className="adminUsers">
                <span>Recent users</span>
                {runtimeStatus.recentUsers.map((user) => (
                  <div key={user.id}>
                    <strong>{user.username}</strong>
                    <small>{user.isAdmin ? "admin" : "user"} · {relativeDate(user.createdAt)}</small>
                  </div>
                ))}
              </div>
            ) : null}
            <div className="clockGrid">
              <div>
                <span>Playing as</span>
                <strong>{colorName(color)}</strong>
              </div>
              <div>
                <span>To move</span>
                <strong>{colorName(turn)}</strong>
              </div>
            </div>
            <div className="resultLine">
              <span>{resultLabel(serverStatus, outcome, method, winner, chess.isCheck())}</span>
            </div>
          </div>

          <div className="panel analysisPanel">
            <div className="panelTitle">
              <Search size={18} />
              Analysis
            </div>
            <button className="analysisButton" onClick={requestAnalysis} disabled={analysisLoading || gameState === "idle" || gameState === "waiting"}>
              {analysisLoading ? "Analyzing..." : "Run Analysis"}
            </button>
            {analysis ? (
              <div className="analysisResult">
                <div>
                  <span>Engine</span>
                  <strong>{analysis.engine}</strong>
                </div>
                <div>
                  <span>Best move</span>
                  <strong>{analysis.bestMove || "none"}</strong>
                </div>
                <div>
                  <span>Score</span>
                  <strong>{analysis.score || "n/a"}</strong>
                </div>
              </div>
            ) : null}
          </div>

          <div className="panel moves">
            <div className="panelTitle">
              <Swords size={18} />
              Scoresheet
            </div>
            <div className="scoreTable">
              {moveBook.length === 0 ? (
                <div className="emptyMoves">No moves yet</div>
              ) : (
                moveBook.map((row) => (
                  <div className="scoreRow" key={row.number}>
                    <span className="moveNumber">{row.number}.</span>
                    <span>{row.white}</span>
                    <span>{row.black}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </aside>
      </section>
      {pendingPromotion ? (
        <div className="promotionOverlay" role="dialog" aria-modal="true">
          <div className="promotionBox">
            <strong>Promote pawn</strong>
            <div className="promotionChoices">
              {promotionPieces.map((piece) => (
                <button
                  key={piece.value}
                  onClick={() => {
                    sendMove(pendingPromotion.from, pendingPromotion.to, piece.value);
                  }}
                >
                  <span>{color === "b" ? piece.black : piece.white}</span>
                  {piece.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      ) : null}
      {selectedGame ? (
        <div className="gameOverlay" role="dialog" aria-modal="true">
          <div className="gameBox replayGameBox">
            <div className="gameBoxHeader">
              <div>
                <strong>{gameResultText(selectedGame)}</strong>
                <span>{gameOpponentText(selectedGame, authUser?.username ?? "")}</span>
              </div>
              <button onClick={() => setSelectedGame(null)}>Close</button>
            </div>
            <div className="detailGrid">
              <span>
                <Server size={14} />
                {selectedGame.mode === "bot" ? selectedGame.aiEngine ?? "heuristic" : "multiplayer"}
              </span>
              <span>{selectedGame.moves.length} moves</span>
              <span>{relativeDate(selectedGame.endedAt)}</span>
              <span>{selectedGame.finalFen ? "FEN saved" : "PGN only"}</span>
            </div>
            <div className="replayPanel">
              <div className="panelTitle">
                <History size={18} />
                Replay
              </div>
              <div className="replayToolbar">
                <button onClick={() => setReplayPly(0)} disabled={replayState.totalPlies === 0 || replayPly === 0}>
                  Start
                </button>
                <button onClick={() => setReplayPly((value) => Math.max(0, value - 1))} disabled={replayState.totalPlies === 0 || replayPly === 0}>
                  Prev
                </button>
                <button
                  onClick={() => setReplayPly((value) => Math.min(replayState.totalPlies, value + 1))}
                  disabled={replayState.totalPlies === 0 || replayPly === replayState.totalPlies}
                >
                  Next
                </button>
                <button onClick={() => setReplayPly(replayState.totalPlies)} disabled={replayState.totalPlies === 0 || replayPly === replayState.totalPlies}>
                  End
                </button>
                <span>
                  {replayPly} / {replayState.totalPlies} plies
                </span>
              </div>
              <div className="replayStatus">
                <span>{replayState.board.turn() === "w" ? "White to move" : "Black to move"}</span>
                <span>
                  {replayState.board.isCheck()
                    ? "Check"
                    : replayState.totalPlies === 0
                      ? "Initial position"
                      : replayPly === replayState.totalPlies
                        ? "Final position"
                        : "Replay position"}
                </span>
              </div>
              <div className="boardFrame replayBoardFrame">
                <BoardView
                  board={replayState.board}
                  orientation={files}
                  ranks={[8, 7, 6, 5, 4, 3, 2, 1]}
                  lastMove={replayState.lastMove}
                  interactive={false}
                />
              </div>
              <div className="replayMoves">
                <div className="panelTitle">
                  <Swords size={18} />
                  Move Replay
                </div>
                {replayMoveRows.length === 0 ? (
                  <div className="emptyMoves">No moves recorded</div>
                ) : (
                  replayMoveRows.map((row) => (
                    <div className={`scoreRow replayRow ${replayPly === row.whitePly || replayPly === row.blackPly ? "activeReplayRow" : ""}`} key={row.number}>
                      <button className={`moveJump ${replayPly === row.whitePly ? "activeReplayMove" : ""}`} onClick={() => row.whitePly && setReplayPly(row.whitePly)} disabled={!row.whitePly}>
                        {row.number}.
                      </button>
                      <button className={`moveJump ${replayPly === row.whitePly ? "activeReplayMove" : ""}`} onClick={() => row.whitePly && setReplayPly(row.whitePly)} disabled={!row.white}>
                        {row.white || "..."}
                      </button>
                      <button className={`moveJump ${replayPly === row.blackPly ? "activeReplayMove" : ""}`} onClick={() => row.blackPly && setReplayPly(row.blackPly)} disabled={!row.black}>
                        {row.black || ""}
                      </button>
                    </div>
                  ))
                )}
              </div>
            </div>
            <pre>{gamePGN(selectedGame)}</pre>
            {selectedGame.finalFen ? <code className="fenLine">{selectedGame.finalFen}</code> : null}
            <small>{selectedGame.moves.join(" ") || "No moves recorded"}</small>
          </div>
        </div>
      ) : null}
      {gameState === "ended" ? (
        <div className="endOverlay" role="dialog" aria-modal="true">
          <div className="endBox">
            <strong>{resultBanner?.title ?? "Game Over"}</strong>
            <span>{resultBanner?.detail ?? status}</span>
            {analysis ? (
              <section className="endAnalysis">
                <p>{analysis.engine}</p>
                <b>{analysis.bestMove || "No move"}</b>
                <small>{analysis.score || "No score"}</small>
              </section>
            ) : null}
            <div>
              <button onClick={requestAnalysis}>
                <Search size={16} />
                Analyze
              </button>
              <button onClick={requestRematch}>
                <RefreshCw size={16} />
                {mode === "bot" ? "Play Again" : "Request Rematch"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}

function PlayerStrip({
  colorLabel,
  active,
  checked,
  bot,
  captures
}: {
  side: "top" | "bottom";
  colorLabel: string;
  active: boolean;
  checked: boolean;
  bot: boolean;
  captures: string[];
}) {
  return (
    <div className={`playerStrip ${active ? "activePlayer" : ""} ${checked ? "checkedPlayer" : ""}`}>
      <div className="avatar">{bot ? "AI" : colorLabel === "White" ? "♔" : "♚"}</div>
      <div>
        <strong>{colorLabel}</strong>
        <span>{bot ? "engine" : checked ? "in check" : active ? "on move" : "waiting"}</span>
        <div className="capturedPieces">{captures.map((piece, index) => <i key={`${piece}-${index}`}>{piece}</i>)}</div>
      </div>
      <Circle size={10} className="turnDot" />
    </div>
  );
}

function BoardView({
  board,
  orientation,
  ranks,
  lastMove,
  selected,
  selectedTargets = new Set<string>(),
  checkedKingSquare = "",
  interactive = true,
  onSquareClick
}: {
  board: Chess;
  orientation: string[];
  ranks: number[];
  lastMove?: string;
  selected?: Square | null;
  selectedTargets?: Set<string>;
  checkedKingSquare?: string;
  interactive?: boolean;
  onSquareClick?: (square: Square) => void;
}) {
  return (
    <>
      <div className="fileLabels topFiles">
        {orientation.map((file) => (
          <span key={`top-${file}`}>{file}</span>
        ))}
      </div>
      <div className="boardRows">
        <div className="rankLabels">
          {ranks.map((rank) => (
            <span key={`left-${rank}`}>{rank}</span>
          ))}
        </div>
        <div className="board">
          {ranks.map((rank) =>
            orientation.map((file) => {
              const square = `${file}${rank}` as Square;
              const piece = board.get(square);
              const light = (files.indexOf(file) + rank) % 2 === 1;
              const isLast = lastMove?.slice(0, 2) === square || lastMove?.slice(2, 4) === square;
              const isTarget = selectedTargets.has(square);
              const isCheckedKing = checkedKingSquare === square;
              return (
                <button
                  className={[
                    "square",
                    light ? "light" : "dark",
                    selected === square ? "selected" : "",
                    isLast ? "lastMove" : "",
                    isTarget ? "target" : "",
                    isCheckedKing ? "checkedKing" : "",
                    interactive ? "" : "replaySquare"
                  ].join(" ")}
                  key={square}
                  onClick={interactive && onSquareClick ? () => onSquareClick(square) : undefined}
                  aria-label={square}
                  disabled={!interactive}
                >
                  <span className={piece ? `piece ${piece.color === "w" ? "whitePiece" : "blackPiece"}` : ""}>
                    {piece ? pieceGlyph(piece.type, piece.color) : ""}
                  </span>
                </button>
              );
            })
          )}
        </div>
        <div className="rankLabels">
          {ranks.map((rank) => (
            <span key={`right-${rank}`}>{rank}</span>
          ))}
        </div>
      </div>
      <div className="fileLabels">
        {orientation.map((file) => (
          <span key={`bottom-${file}`}>{file}</span>
        ))}
      </div>
    </>
  );
}

function colorName(value?: "w" | "b" | "") {
  if (value === "w") return "White";
  if (value === "b") return "Black";
  return "Unassigned";
}

function opponentLabel(color: "w" | "b" | undefined, mode: "multiplayer" | "bot", aiColor?: "w" | "b") {
  if (mode === "bot") return `AI ${colorName(aiColor)}`;
  return color === "b" ? "White" : "Black";
}

function gameStatusLabel(value: string) {
  const labels: Record<string, string> = {
    idle: "Ready",
    waiting: "Seeking",
    playing: "Playing",
    ended: "Ended",
    offline: "Offline"
  };
  return labels[value] ?? value;
}

function nextActionText(gameState: string, roomCode: string) {
  if (gameState === "waiting" && roomCode) return `Share room code ${roomCode} with another player.`;
  if (gameState === "waiting") return "Waiting for another player to join the queue.";
  return "Start a public match, create a private room, or play against AI.";
}

function gameResultText(game: GameRecord) {
  if (game.outcome === "1-0") return "White won";
  if (game.outcome === "0-1") return "Black won";
  if (game.outcome === "1/2-1/2") return "Draw";
  if (game.winner) return `${colorName(game.winner)} won`;
  return game.method && game.method !== "NoMethod" ? game.method : "Completed";
}

function gameOpponentText(game: GameRecord, username: string) {
  const white = game.whiteUsername || "White";
  const black = game.blackUsername || (game.mode === "bot" ? "AI" : "Black");
  const opponent = white === username ? black : white;
  const mode = game.mode === "bot" ? `AI ${game.aiLevel ?? ""}`.trim() : "Multiplayer";
  return `${mode} vs ${opponent}`;
}

function gamePGN(game: GameRecord) {
  const board = new Chess();
  const sanMoves = game.moves.map((moveText) => {
    const move = playMoveText(board, moveText);
    return move?.san ?? moveText;
  });
  const moveText = sanMoves
    .reduce<string[]>((rows, move, index) => {
      if (index % 2 === 0) rows.push(`${Math.floor(index / 2) + 1}. ${move}`);
      else rows[rows.length - 1] += ` ${move}`;
      return rows;
    }, [])
    .join(" ");
  const white = game.whiteUsername || "White";
  const black = game.blackUsername || (game.mode === "bot" ? "AI" : "Black");
  const result = game.outcome || "*";
  return [
    `[White "${white}"]`,
    `[Black "${black}"]`,
    `[Result "${result}"]`,
    `[Date "${formatPGNDate(game.endedAt)}"]`,
    "",
    `${moveText} ${result}`.trim()
  ].join("\n");
}

function formatPGNDate(value: string) {
  if (!value) return "????.??.??";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "????.??.??";
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}.${month}.${day}`;
}

function relativeDate(value: string) {
  if (!value) return "just now";
  const elapsed = Date.now() - new Date(value).getTime();
  if (elapsed < 60_000) return "just now";
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function statusText(status?: string, method?: string, winner?: "w" | "b" | "") {
  if (status === "resignation") return `${colorName(winner)} wins by resignation`;
  if (status === "checkmate") return `${colorName(winner)} wins by checkmate`;
  if (status === "draw") return method && method !== "NoMethod" ? `Draw by ${method}` : "Draw";
  return "Game in progress";
}

function resultLabel(status: string, outcome: string, method: string, winner: "w" | "b" | "", inCheck: boolean) {
  if (status === "active") return inCheck ? "Check" : "Position is active";
  if (status === "draw") return method === "NoMethod" ? "Draw" : `Draw by ${method}`;
  if (winner) return `${colorName(winner)} wins (${outcome}) by ${method}`;
  return status;
}

function resultBannerText(status: string, method: string, winner: "w" | "b" | "", inCheck: boolean) {
  if (status === "active" && inCheck) {
    return { title: "Check", detail: "King is under attack" };
  }
  if (status === "checkmate") {
    return { title: "Checkmate", detail: `${colorName(winner)} wins` };
  }
  if (status === "resignation") {
    return { title: "Resignation", detail: `${colorName(winner)} wins` };
  }
  if (status === "draw") {
    return { title: "Draw", detail: method === "NoMethod" ? "Game drawn" : method };
  }
  return null;
}

function findKingSquare(chess: Chess, color: "w" | "b") {
  for (const rank of [1, 2, 3, 4, 5, 6, 7, 8]) {
    for (const file of files) {
      const square = `${file}${rank}` as Square;
      const piece = chess.get(square);
      if (piece?.type === "k" && piece.color === color) return square;
    }
  }
  return "";
}

function needsPromotion(chess: Chess, from: Square, to: Square) {
  const piece = chess.get(from);
  if (piece?.type !== "p") return false;
  return (piece.color === "w" && to.endsWith("8")) || (piece.color === "b" && to.endsWith("1"));
}

function buildMoveBook(moves: string[]) {
  const board = new Chess();
  const rows: { number: number; white: string; black: string; whitePly: number; blackPly: number }[] = [];

  moves.forEach((moveText, index) => {
    const move = playMoveText(board, moveText);
    const san = move?.san ?? moveText;
    const rowIndex = Math.floor(index / 2);
    if (!rows[rowIndex]) rows[rowIndex] = { number: rowIndex + 1, white: "", black: "", whitePly: 0, blackPly: 0 };
    if (index % 2 === 0) {
      rows[rowIndex].white = san;
      rows[rowIndex].whitePly = index + 1;
    } else {
      rows[rowIndex].black = san;
      rows[rowIndex].blackPly = index + 1;
    }
  });

  return rows;
}

function buildReplayState(game: GameRecord | null, ply: number) {
  const board = new Chess();
  const totalPlies = game?.moves.length ?? 0;
  const safePly = Math.max(0, Math.min(ply, totalPlies));
  let lastMove = "";

  (game?.moves ?? []).slice(0, safePly).forEach((moveText) => {
    const move = playMoveText(board, moveText);
    if (move) lastMove = moveText;
  });

  return { board, lastMove, totalPlies, safePly };
}

function playMoveText(board: Chess, moveText: string) {
  return board.move({
    from: moveText.slice(0, 2),
    to: moveText.slice(2, 4),
    promotion: moveText.slice(4) || "q"
  });
}

function buildCapturedPieces(moves: string[], color?: "w" | "b") {
  const board = new Chess();
  const byWhite: string[] = [];
  const byBlack: string[] = [];

  moves.forEach((moveText) => {
    const move = board.move({
      from: moveText.slice(0, 2),
      to: moveText.slice(2, 4),
      promotion: moveText.slice(4) || "q"
    });
    if (!move?.captured) return;
    const capturedColor = move.color === "w" ? "b" : "w";
    const glyph = pieceGlyph(move.captured, capturedColor);
    if (move.color === "w") byWhite.push(glyph);
    else byBlack.push(glyph);
  });

  return {
    byYou: color === "b" ? byBlack : byWhite,
    byOpponent: color === "b" ? byWhite : byBlack
  };
}

function pieceGlyph(type: string, color: "w" | "b") {
  const glyphs: Record<string, string> = {
    wp: "♙",
    wn: "♘",
    wb: "♗",
    wr: "♖",
    wq: "♕",
    wk: "♔",
    bp: "♟",
    bn: "♞",
    bb: "♝",
    br: "♜",
    bq: "♛",
    bk: "♚"
  };
  return glyphs[`${color}${type}`] ?? "";
}

createRoot(document.getElementById("root")!).render(<App />);
