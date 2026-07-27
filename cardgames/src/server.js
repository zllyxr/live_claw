/**
 * 牌类游戏服务：斗地主 + 麻将
 *
 * 服务端权威：所有发牌、判定、AI 决策都在这里完成，
 * 客户端只提交动作并渲染服务端返回的视图。
 *
 * 支持 BASE_PATH 子路径挂载，便于经 Apache 反代到 /minigame/xxx/。
 */
import { createServer as createHttpServer } from 'node:http';
import { randomUUID } from 'node:crypto';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import express from 'express';
import { Server as SocketIOServer } from 'socket.io';

import * as ddz from './ddz/game.js';
import * as mahjong from './mahjong/game.js';
import * as paodekuai from './paodekuai/game.js';
import * as zhajinhua from './zhajinhua/game.js';
import { createBotPlayer, isBotPlayer, playBotTurn } from './bots.js';
import { BUY_IN, PlatformWallet, TABLE_COUNT, verifyLaunchTicket } from './platform.js';

const here = dirname(fileURLToPath(import.meta.url));
const publicDir = join(here, '..', 'public');

/** 归一化基础路径：'' 或 '/minigame/cards'（无尾斜杠） */
function normalizeBasePath(value) {
  const raw = String(value || '').trim();
  if (!raw || raw === '/') return '';
  const withLead = raw.startsWith('/') ? raw : `/${raw}`;
  return withLead.replace(/\/+$/, '');
}

function normalizeDelay(value, fallback) {
  const delay = Number(value);
  return Number.isFinite(delay) ? Math.max(0, Math.min(30_000, Math.floor(delay))) : fallback;
}

/** 内存会话存储。带容量上限与过期清理，避免长期运行内存无界增长。 */
class SessionStore {
  constructor({ maxSessions = 500, ttlMs = 2 * 60 * 60 * 1000 } = {}) {
    this.map = new Map();
    this.maxSessions = maxSessions;
    this.ttlMs = ttlMs;
  }

  create(game) {
    this.sweep();
    if (this.map.size >= this.maxSessions) {
      // 淘汰最旧的一个
      const oldest = [...this.map.entries()].sort((a, b) => a[1].touchedAt - b[1].touchedAt)[0];
      if (oldest) this.map.delete(oldest[0]);
    }
    const id = randomUUID();
    game.id = id;
    this.map.set(id, { game, touchedAt: Date.now() });
    return game;
  }

  get(id) {
    const entry = this.map.get(String(id || ''));
    if (!entry) return null;
    entry.touchedAt = Date.now();
    return entry.game;
  }

  sweep() {
    const now = Date.now();
    for (const [id, entry] of this.map) {
      if (now - entry.touchedAt > this.ttlMs) this.map.delete(id);
    }
  }

  get size() {
    return this.map.size;
  }
}

export function createCardGameServer(options = {}) {
  const app = express();
  const httpServer = createHttpServer(app);
  const basePath = normalizeBasePath(options.basePath ?? process.env.BASE_PATH ?? '');
  const secret = String(options.secret ?? process.env.MINIGAME_SECRET ?? '');
  const authRequired = options.authRequired ?? Boolean(secret);
  const wallet = options.wallet ?? new PlatformWallet(options.walletOptions);
  const io = new SocketIOServer(httpServer, {
    path: `${basePath}/socket.io`,
    transports: ['websocket', 'polling'],
    maxHttpBufferSize: 32 * 1024
  });
  const sessions = new SessionStore(options.sessionOptions);
  const tables = new Map();
  const startedAt = Date.now();
  const timers = new Set();
  let closing = false;
  const botsEnabled = options.botsEnabled
    ?? String(process.env.BOT_FILL_ENABLED ?? '1') !== '0';
  const botFillDelayMs = normalizeDelay(
    options.botFillDelayMs ?? process.env.BOT_FILL_DELAY_MS,
    1500
  );
  const legacyBotActionDelay = options.botActionDelayMs ?? process.env.BOT_ACTION_DELAY_MS;
  const botActionMinMs = normalizeDelay(
    options.botActionMinMs ?? process.env.BOT_ACTION_MIN_MS ?? legacyBotActionDelay,
    900
  );
  const botActionMaxMs = Math.max(botActionMinMs, normalizeDelay(
    options.botActionMaxMs ?? process.env.BOT_ACTION_MAX_MS ?? legacyBotActionDelay,
    2600
  ));
  const humanTurnTimeoutMs = normalizeDelay(
    options.humanTurnTimeoutMs ?? process.env.HUMAN_TURN_TIMEOUT_MS,
    15_000
  );
  const humanClaimTimeoutMs = Math.min(humanTurnTimeoutMs, normalizeDelay(
    options.humanClaimTimeoutMs ?? process.env.HUMAN_CLAIM_TIMEOUT_MS,
    8_000
  ));
  const random = typeof options.random === 'function' ? options.random : Math.random;
  const botStats = {
    rounds: 0,
    houseNet: 0
  };

  function scheduleTimer(callback, delay) {
    if (closing) return null;
    const timer = setTimeout(() => {
      timers.delete(timer);
      Promise.resolve(callback()).catch((error) => options.logger?.error?.(error));
    }, delay);
    timers.add(timer);
    return timer;
  }

  function cancelTimer(timer) {
    if (!timer) return;
    clearTimeout(timer);
    timers.delete(timer);
  }

  app.disable('x-powered-by');
  app.use(express.json({ limit: '32kb' }));

  // 静态资源：根路径与子路径都挂，直连与反代都能用
  app.use(express.static(publicDir));
  if (basePath) app.use(basePath, express.static(publicDir));

  const healthPaths = basePath ? ['/health', `${basePath}/health`] : ['/health'];
  app.get(healthPaths, (_req, res) => {
    res.json({
      ok: true,
      status: 'ok',
      uptimeSeconds: Math.floor((Date.now() - startedAt) / 1000),
      sessions: sessions.size,
      activeTables: tables.size,
      tableCount: TABLE_COUNT,
      humanPlayers: [...tables.values()]
        .reduce((sum, table) => sum + [...table.players.values()].filter((p) => !isBotPlayer(p)).length, 0),
      botPlayers: [...tables.values()]
        .reduce((sum, table) => sum + [...table.players.values()].filter(isBotPlayer).length, 0),
      botsEnabled,
      botFillDelayMs,
      botActionMinMs,
      botActionMaxMs,
      humanTurnTimeoutMs,
      humanClaimTimeoutMs,
      botRounds: botStats.rounds,
      botHouseNet: botStats.houseNet,
      games: ['ddz', 'mahjong', 'mahjong_red', 'paodekuai', 'zhajinhua']
    });
  });

  const lobbyPaths = basePath
    ? ['/api/lobby/:kind', `${basePath}/api/lobby/:kind`]
    : ['/api/lobby/:kind'];
  app.get(lobbyPaths, (req, res) => {
    const kind = String(req.params.kind || '');
    if (!['ddz', 'mahjong', 'mahjong_red', 'paodekuai', 'zhajinhua'].includes(kind)) {
      res.status(404).json({ ok: false, message: '游戏不存在' });
      return;
    }
    const active = [...tables.values()].filter((table) => table.kind === kind);
    res.json({
      ok: true,
      game: kind,
      entryMode: 'match',
      tableCount: TABLE_COUNT,
      activeTables: active.length,
      onlinePlayers: active.reduce(
        (sum, table) => sum + [...table.players.values()].filter((p) => !isBotPlayer(p)).length,
        0
      ),
      botPlayers: active.reduce(
        (sum, table) => sum + [...table.players.values()].filter(isBotPlayer).length,
        0
      )
    });
  });

  /** 统一包装：捕获规则层抛出的错误，返回结构化响应 */
  const handle = (fn) => (req, res) => {
    try {
      const payload = fn(req);
      res.json({ ok: true, ...payload });
    } catch (error) {
      res.status(400).json({ ok: false, message: error?.message || '操作失败' });
    }
  };

  const requireGame = (req, kind) => {
    const game = sessions.get(req.body?.gameId ?? req.query?.gameId);
    if (!game) throw new Error('对局不存在或已过期，请重新开局');
    if (game.kind !== kind) throw new Error('对局类型不匹配');
    return game;
  };

  const routes = (prefix) => {
    /* ---------------- 斗地主 ---------------- */
    app.post(`${prefix}/api/ddz/new`, handle(() => {
      const game = sessions.create(ddz.createGame());
      game.kind = 'ddz';
      return { state: ddz.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/ddz/bid`, handle((req) => {
      const game = requireGame(req, 'ddz');
      ddz.bid(game, 0, Boolean(req.body?.call));
      return { state: ddz.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/ddz/play`, handle((req) => {
      const game = requireGame(req, 'ddz');
      const cards = Array.isArray(req.body?.cards) ? req.body.cards.map(Number) : [];
      ddz.play(game, 0, cards);
      return { state: ddz.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/ddz/pass`, handle((req) => {
      const game = requireGame(req, 'ddz');
      ddz.pass(game, 0);
      return { state: ddz.viewFor(game, 0) };
    }));

    app.get(`${prefix}/api/ddz/state`, handle((req) => {
      const game = requireGame(req, 'ddz');
      return { state: ddz.viewFor(game, 0) };
    }));

    /* ---------------- 麻将 ---------------- */
    app.post(`${prefix}/api/mahjong/new`, handle(() => {
      const game = sessions.create(mahjong.createGame());
      game.kind = 'mahjong';
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/discard`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.discard(game, 0, Number(req.body?.tile));
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/peng`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.peng(game, 0);
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/gang`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.gang(game, 0);
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/concealed-gang`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.concealedGang(game, 0, Number(req.body?.tile));
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/win`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.declare(game, 0);
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.post(`${prefix}/api/mahjong/skip`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      mahjong.skipClaim(game);
      return { state: mahjong.viewFor(game, 0) };
    }));

    app.get(`${prefix}/api/mahjong/state`, handle((req) => {
      const game = requireGame(req, 'mahjong');
      return { state: mahjong.viewFor(game, 0) };
    }));
  };

  routes('');
  if (basePath) routes(basePath);

  const roomName = (kind, tableNo) => `cards:${kind}:${tableNo}`;
  const seatsFor = (kind) => ['ddz', 'paodekuai', 'zhajinhua'].includes(kind) ? 3 : 4;

  function tableFor(kind, tableNo) {
    const key = `${kind}:${tableNo}`;
    let table = tables.get(key);
    if (!table) {
      table = {
        key,
        kind,
        tableNo,
        players: new Map(),
        game: null,
        roundNo: '',
        status: 'waiting',
        ready: new Set(),
        settled: false,
        botFillTimer: null,
        botActionTimer: null,
        humanActionTimer: null,
        turnDeadline: null,
        turnSeat: null,
        turnToken: 0
      };
      tables.set(key, table);
    }
    return table;
  }

  function waitingView(table) {
    return {
      game: table.kind,
      tableNo: table.tableNo,
      tableCount: TABLE_COUNT,
      entryMode: 'match',
      status: table.status,
      players: [...table.players.values()]
        .sort((a, b) => a.seat - b.seat)
        .map(({ uid, name, seat, balance, isBot, connected }) => ({
          uid,
          name,
          seat,
          balance,
          isBot: Boolean(isBot),
          connected: Boolean(isBot) || Boolean(connected)
        })),
      requiredPlayers: seatsFor(table.kind),
      buyIn: BUY_IN
    };
  }

  function emitWaiting(table) {
    io.to(roomName(table.kind, table.tableNo)).emit('match:state', waitingView(table));
  }

  function emitGameStates(table) {
    for (const player of table.players.values()) {
      const socket = io.sockets.sockets.get(player.socketId);
      if (!socket || !table.game) continue;
      let state;
      if (table.kind === 'ddz') state = ddz.viewFor(table.game, player.seat);
      else if (table.kind === 'paodekuai') state = paodekuai.viewFor(table.game, player.seat);
      else if (table.kind === 'zhajinhua') state = zhajinhua.viewFor(table.game, player.seat);
      else state = mahjong.viewFor(table.game, player.seat);
      socket.emit('game:state', {
        ...state,
        game: table.kind,
        tableNo: table.tableNo,
        tableCount: TABLE_COUNT,
        entryMode: 'match',
        buyIn: BUY_IN,
        walletBalance: player.balance,
        turnSeat: table.turnSeat,
        turnDeadline: table.turnDeadline,
        turnTimeoutSeconds: table.turnDeadline
          ? Math.max(0, Math.ceil((table.turnDeadline - Date.now()) / 1000))
          : 0
      });
    }
  }

  function humansAt(table) {
    return [...table.players.values()].filter((player) => !isBotPlayer(player));
  }

  function cancelBotFill(table) {
    cancelTimer(table.botFillTimer);
    table.botFillTimer = null;
  }

  function cancelBotAction(table) {
    cancelTimer(table.botActionTimer);
    table.botActionTimer = null;
  }

  function cancelTurnAction(table) {
    cancelBotAction(table);
    cancelTimer(table.humanActionTimer);
    table.humanActionTimer = null;
    table.turnDeadline = null;
    table.turnSeat = null;
    table.turnToken += 1;
  }

  function actionSeatFor(table) {
    if (!table.game || table.game.phase === 'finished') return null;
    if (table.kind === 'ddz' && table.game.phase === 'bidding') return table.game.bidTurn;
    if ((table.kind === 'mahjong' || table.kind === 'mahjong_red') && table.game.pendingClaim) {
      return table.game.pendingClaim.seat;
    }
    return table.game.current;
  }

  function playerAtSeat(table, seat) {
    return [...table.players.values()].find((player) => player.seat === seat) || null;
  }

  function randomBotActionDelay() {
    if (botActionMaxMs === botActionMinMs) return botActionMinMs;
    return botActionMinMs + Math.floor(random() * (botActionMaxMs - botActionMinMs + 1));
  }

  function playTimedOutHumanTurn(table, seat) {
    table.game.history.push({ type: 'timeout', seat });
    if (table.kind === 'ddz') {
      if (table.game.phase === 'bidding') {
        ddz.bid(table.game, seat, false);
        return;
      }
      const view = ddz.viewFor(table.game, seat);
      if (view.canPass) ddz.pass(table.game, seat);
      else if (view.hints[0]) ddz.play(table.game, seat, view.hints[0]);
      return;
    }
    if (table.kind === 'paodekuai') {
      const view = paodekuai.viewFor(table.game, seat);
      if (view.canPass) paodekuai.pass(table.game, seat);
      else if (view.hints[0]) paodekuai.play(table.game, seat, view.hints[0]);
      return;
    }
    if (table.kind === 'zhajinhua') {
      const view = zhajinhua.viewFor(table.game, seat);
      zhajinhua.act(table.game, seat, { action: view.canCheck ? 'check' : 'fold' });
      return;
    }
    if (table.game.pendingClaim?.seat === seat) {
      mahjong.skipClaim(table.game, seat);
      return;
    }
    const hand = table.game.hands[seat];
    const tile = table.game.drawnTile ?? hand[hand.length - 1];
    mahjong.discard(table.game, seat, tile);
  }

  async function fillBots(table) {
    table.botFillTimer = null;
    if (!botsEnabled || table.status !== 'waiting' || humansAt(table).length === 0) return;
    const required = seatsFor(table.kind);
    const occupied = new Set([...table.players.values()].map((player) => player.seat));
    for (let seat = 0; seat < required; seat += 1) {
      if (occupied.has(seat)) continue;
      const bot = createBotPlayer({
        kind: table.kind,
        tableNo: table.tableNo,
        seat,
        buyIn: BUY_IN
      });
      table.players.set(bot.uid, bot);
    }
    emitWaiting(table);
    await startTable(table);
  }

  function scheduleBotFill(table) {
    if (!botsEnabled || table.botFillTimer || table.status !== 'waiting') return;
    if (humansAt(table).length === 0 || table.players.size >= seatsFor(table.kind)) return;
    table.botFillTimer = scheduleTimer(() => fillBots(table), botFillDelayMs);
  }

  function scheduleTurnAction(table) {
    cancelTurnAction(table);
    if (closing) return;
    if (table.status !== 'playing' || !table.game || table.game.phase === 'finished') return;
    const seat = actionSeatFor(table);
    const actor = playerAtSeat(table, seat);
    if (!actor) return;
    const isClaim = Boolean(
      (table.kind === 'mahjong' || table.kind === 'mahjong_red')
      && table.game.pendingClaim
    );
    const automated = isBotPlayer(actor) || actor.connected === false;
    const delay = automated
      ? randomBotActionDelay()
      : (isClaim ? humanClaimTimeoutMs : humanTurnTimeoutMs);
    const token = table.turnToken;
    table.turnSeat = seat;
    table.turnDeadline = Date.now() + delay;
    const run = async () => {
      if (token !== table.turnToken || table.status !== 'playing' || actionSeatFor(table) !== seat) return;
      table.botActionTimer = null;
      table.humanActionTimer = null;
      if (isBotPlayer(actor)) playBotTurn(table.kind, table.game, seat);
      else playTimedOutHumanTurn(table, seat);
      if (table.game.phase === 'finished') {
        cancelTurnAction(table);
        table.status = 'finished';
        emitGameStates(table);
        await settleTable(table);
        return;
      }
      scheduleTurnAction(table);
      emitGameStates(table);
    };
    if (automated) table.botActionTimer = scheduleTimer(run, delay);
    else table.humanActionTimer = scheduleTimer(run, delay);
  }

  async function settleTable(table) {
    if (!table.game?.result || table.settled) return;
    cancelTurnAction(table);
    const scores = table.game.result.scores || [0, 0, 0, 0];
    const payouts = table.game.result.payouts;
    let botHouseNet = 0;
    for (const player of [...table.players.values()].sort((a, b) => a.seat - b.seat)) {
      const payout = Array.isArray(payouts)
        ? Number(payouts[player.seat] || 0)
        : BUY_IN + Number(scores[player.seat] || 0);
      if (isBotPlayer(player)) {
        player.balance = payout;
        botHouseNet += BUY_IN - payout;
        continue;
      }
      if (payout === 0) continue;
      const result = await wallet.adjust({
        order_no: `${table.roundNo}:payout:${player.uid}`,
        uid: player.uid,
        game_code: table.kind,
        table_no: table.tableNo,
        round_no: table.roundNo,
        reason: 'round_payout',
        amount: payout
      });
      player.balance = result.balance;
    }
    table.settled = true;
    if ([...table.players.values()].some(isBotPlayer)) {
      botStats.rounds += 1;
      botStats.houseNet += botHouseNet;
    }
    emitGameStates(table);
    io.to(roomName(table.kind, table.tableNo)).emit('wallet:settled', {
      roundNo: table.roundNo,
      balances: humansAt(table).map((p) => ({ uid: p.uid, balance: p.balance })),
      botHouseNet
    });
  }

  async function startTable(table) {
    if (table.status === 'starting' || table.status === 'playing') return;
    if (table.players.size !== seatsFor(table.kind)) return;
    cancelBotFill(table);
    cancelTurnAction(table);
    table.status = 'starting';
    table.roundNo = `${table.kind}-${table.tableNo}-${randomUUID()}`;
    table.settled = false;
    emitWaiting(table);

    const debited = [];
    try {
      for (const player of [...table.players.values()].sort((a, b) => a.seat - b.seat)) {
        if (isBotPlayer(player)) {
          player.balance = BUY_IN;
          continue;
        }
        const result = await wallet.adjust({
          order_no: `${table.roundNo}:buyin:${player.uid}`,
          uid: player.uid,
          game_code: table.kind,
          table_no: table.tableNo,
          round_no: table.roundNo,
          reason: 'round_buy_in',
          amount: -BUY_IN
        });
        player.balance = result.balance;
        debited.push(player);
      }
    } catch (error) {
      for (const player of debited) {
        try {
          const refund = await wallet.adjust({
            order_no: `${table.roundNo}:refund:${player.uid}`,
            uid: player.uid,
            game_code: table.kind,
            table_no: table.tableNo,
            round_no: table.roundNo,
            reason: 'start_failed_refund',
            amount: BUY_IN
          });
          player.balance = refund.balance;
        } catch {}
      }
      table.status = 'waiting';
      io.to(roomName(table.kind, table.tableNo)).emit('match:error', {
        message: error?.message || '有玩家余额不足，暂时无法开局'
      });
      emitWaiting(table);
      return;
    }

    const ordered = [...table.players.values()].sort((a, b) => a.seat - b.seat);
    const gameOptions = {
      humanSeats: ordered.filter((p) => !isBotPlayer(p)).map((p) => p.seat),
      seatNames: ordered.map((p) => p.name),
      baseStake: 1,
      deferBots: true
    };
    if (table.kind === 'ddz') table.game = ddz.createGame(gameOptions);
    else if (table.kind === 'paodekuai') table.game = paodekuai.createGame(gameOptions);
    else if (table.kind === 'zhajinhua') table.game = zhajinhua.createGame(gameOptions);
    else if (table.kind === 'mahjong_red') {
      table.game = mahjong.createGame({ ...gameOptions, ruleset: 'red-center', kind: 'mahjong_red' });
    } else {
      table.game = mahjong.createGame(gameOptions);
    }
    table.game.id = table.roundNo;
    table.status = 'playing';
    table.ready.clear();
    scheduleTurnAction(table);
    emitGameStates(table);
  }

  function parseIdentity(payload, kind) {
    if (authRequired) return verifyLaunchTicket(payload, kind, secret);
    const uid = Number(payload?.uid);
    const table = Number(payload?.table);
    if (!Number.isSafeInteger(uid) || uid < 1) throw new Error('玩家身份无效');
    if (!Number.isInteger(table) || table < 1 || table > TABLE_COUNT) throw new Error('匹配桌号无效');
    return { uid, table, name: String(payload?.name || `玩家${uid}`).slice(0, 20) };
  }

  io.on('connection', (socket) => {
    socket.on('match:join', async (payload = {}, ack = () => {}) => {
      try {
        const kind = String(payload.game || '');
        if (!['ddz', 'mahjong', 'mahjong_red', 'paodekuai', 'zhajinhua'].includes(kind)) throw new Error('游戏不存在');
        const identity = parseIdentity(payload, kind);
        const table = tableFor(kind, identity.table);
        const existing = table.players.get(identity.uid);
        let replaceableBot = null;
        if (!existing && table.players.size >= seatsFor(kind)) {
          if (table.status === 'waiting' || table.status === 'finished') {
            replaceableBot = [...table.players.values()].find(isBotPlayer) || null;
          }
          if (!replaceableBot) throw new Error('当前桌已开局，请返回平台重新匹配');
        }
        const walletState = await wallet.balance(identity.uid);
        if (!existing && walletState.balance < BUY_IN) {
          throw new Error(`钱包余额不足，开局至少需要 ${BUY_IN}`);
        }

        let player = existing;
        if (player) {
          const oldSocket = io.sockets.sockets.get(player.socketId);
          oldSocket?.leave(roomName(kind, table.tableNo));
          player.socketId = socket.id;
          player.name = identity.name;
          player.balance = walletState.balance;
          player.connected = true;
        } else {
          let seat;
          if (replaceableBot) {
            table.players.delete(replaceableBot.uid);
            seat = replaceableBot.seat;
          } else {
            const occupied = new Set([...table.players.values()].map((p) => p.seat));
            seat = [...Array(seatsFor(kind)).keys()].find((candidate) => !occupied.has(candidate));
          }
          player = {
            ...identity,
            seat,
            socketId: socket.id,
            balance: walletState.balance,
            isBot: false,
            connected: true
          };
          table.players.set(identity.uid, player);
        }
        socket.data.tableKey = table.key;
        socket.data.uid = identity.uid;
        socket.join(roomName(kind, table.tableNo));
        ack({ ok: true, ...waitingView(table), seat: player.seat, balance: player.balance });
        if (table.game && (table.status === 'playing' || table.status === 'finished')) {
          if (table.status === 'playing' && actionSeatFor(table) === player.seat) {
            scheduleTurnAction(table);
          }
          emitGameStates(table);
        } else {
          emitWaiting(table);
          await startTable(table);
          if (table.status === 'waiting') scheduleBotFill(table);
        }
      } catch (error) {
        ack({ ok: false, message: error?.message || '匹配失败' });
      }
    });

    socket.on('game:action', async (payload = {}, ack = () => {}) => {
      try {
        const table = tables.get(socket.data.tableKey);
        const player = table?.players.get(socket.data.uid);
        if (!table || !player || !table.game || table.status !== 'playing') throw new Error('对局尚未开始');
        const action = String(payload.action || '');
        if (table.kind === 'ddz') {
          if (action === 'bid') ddz.bid(table.game, player.seat, Boolean(payload.call));
          else if (action === 'play') ddz.play(table.game, player.seat, Array.isArray(payload.cards) ? payload.cards.map(Number) : []);
          else if (action === 'pass') ddz.pass(table.game, player.seat);
          else throw new Error('未知操作');
        } else if (table.kind === 'paodekuai') {
          if (action === 'play') paodekuai.play(table.game, player.seat, Array.isArray(payload.cards) ? payload.cards.map(Number) : []);
          else if (action === 'pass') paodekuai.pass(table.game, player.seat);
          else throw new Error('未知操作');
        } else if (table.kind === 'zhajinhua') {
          zhajinhua.act(table.game, player.seat, {
            action,
            amount: payload.amount,
            target: payload.target
          });
        } else {
          if (action === 'discard') mahjong.discard(table.game, player.seat, Number(payload.tile));
          else if (action === 'peng') mahjong.peng(table.game, player.seat);
          else if (action === 'gang') mahjong.gang(table.game, player.seat);
          else if (action === 'concealed-gang') mahjong.concealedGang(table.game, player.seat, Number(payload.tile));
          else if (action === 'win') mahjong.declare(table.game, player.seat);
          else if (action === 'skip') mahjong.skipClaim(table.game, player.seat);
          else throw new Error('未知操作');
        }
        if (table.game.phase === 'finished') {
          cancelTurnAction(table);
          table.status = 'finished';
          emitGameStates(table);
          ack({ ok: true });
          await settleTable(table);
        } else {
          scheduleTurnAction(table);
          emitGameStates(table);
          ack({ ok: true });
        }
      } catch (error) {
        ack({ ok: false, message: error?.message || '操作失败' });
      }
    });

    socket.on('match:ready', async (_payload = {}, ack = () => {}) => {
      const table = tables.get(socket.data.tableKey);
      const player = table?.players.get(socket.data.uid);
      if (!table || !player || table.status !== 'finished' || !table.settled) {
        ack({ ok: false, message: '本局尚未完成结算' });
        return;
      }
      table.ready.add(player.uid);
      ack({ ok: true, ready: table.ready.size });
      const requiredReady = humansAt(table).length;
      if (requiredReady > 0 && table.ready.size >= requiredReady) {
        cancelTurnAction(table);
        table.game = null;
        table.status = 'waiting';
        await startTable(table);
      } else {
        emitWaiting(table);
      }
    });

    const leaveCurrentTable = () => {
      const table = tables.get(socket.data.tableKey);
      const player = table?.players.get(socket.data.uid);
      if (!table || !player || player.socketId !== socket.id) return null;
      socket.leave(roomName(table.kind, table.tableNo));
      if (table.status === 'waiting' && !table.game) {
        table.players.delete(player.uid);
        table.ready.delete(player.uid);
        emitWaiting(table);
        if (humansAt(table).length === 0) cancelBotFill(table);
        return { mode: 'left' };
      }
      player.connected = false;
      player.socketId = null;
      if (table.status === 'playing') {
        table.game.history.push({ type: 'offline', seat: player.seat });
        if (!closing && actionSeatFor(table) === player.seat) scheduleTurnAction(table);
        emitGameStates(table);
      }
      return { mode: 'trustee' };
    };

    socket.on('match:leave', (_payload = {}, ack = () => {}) => {
      const result = leaveCurrentTable();
      ack({ ok: true, ...(result || { mode: 'none' }) });
    });

    socket.on('disconnect', () => {
      leaveCurrentTable();
    });
  });

  return {
    app,
    httpServer,
    io,
    sessions,
    tables,
    basePath,
    listen(port = Number(process.env.PORT) || 3000) {
      return new Promise((resolve) => {
        httpServer.listen(port, () => resolve(httpServer));
      });
    },
    close() {
      closing = true;
      for (const timer of timers) clearTimeout(timer);
      timers.clear();
      return new Promise((resolve) => io.close(() => {
        if (httpServer.listening) httpServer.close(resolve);
        else resolve();
      }));
    }
  };
}

// 直接运行时启动
if (process.argv[1] && process.argv[1].endsWith('server.js')) {
  const server = createCardGameServer();
  const port = Number(process.env.PORT) || 3000;
  server.listen(port).then(() => {
    const suffix = server.basePath || '/';
    console.log(`[cardgames] listening on ${port}, base path ${suffix}`);
  });
}
