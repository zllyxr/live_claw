import { createServer as createHttpServer } from 'node:http';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import express from 'express';
import { Server as SocketIOServer } from 'socket.io';
import {
  GameEngine,
  SEATS,
  SIMULATION_HZ,
  SNAPSHOT_INTERVAL_MS,
} from './engine.js';
import { PlatformWallet, TABLE_COUNT, verifyLaunchTicket } from './platform.js';

const moduleDirectory = path.dirname(fileURLToPath(import.meta.url));
const defaultPublicDirectory = path.resolve(moduleDirectory, '../public');

function socketRoom(roomId) {
  return `fishing:${roomId}`;
}

function safeAck(ack) {
  return typeof ack === 'function' ? ack : () => {};
}

function connectionError(commandId = null) {
  return {
    ok: false,
    commandId,
    error: { code: 'NOT_JOINED', message: '请先加入房间' },
  };
}

function allowSameOriginRequest(request, callback) {
  const origin = request.headers.origin;
  if (!origin) {
    callback(null, true);
    return;
  }
  const forwardedHost = String(request.headers['x-forwarded-host'] || '').split(',')[0].trim();
  const requestHost = forwardedHost || String(request.headers.host || '').trim();
  try {
    callback(null, Boolean(requestHost) && new URL(origin).host === requestHost);
  } catch {
    callback(null, false);
  }
}

/** 归一化基础路径：'' 或 '/minigame/fish'（无尾斜杠） */
function normalizeBasePath(value) {
  const raw = String(value || '').trim();
  if (!raw || raw === '/') return '';
  const withLead = raw.startsWith('/') ? raw : `/${raw}`;
  return withLead.replace(/\/+$/, '');
}

export function createGameServer(options = {}) {
  const app = express();
  const httpServer = createHttpServer(app);
  // 子路径挂载支持：反向代理到 /minigame/<code>/ 时，静态资源与 socket.io
  // 都需要带上同一前缀，否则客户端会向站点根请求资源而 404。
  const basePath = normalizeBasePath(options.basePath ?? process.env.BASE_PATH ?? '');
  const secret = String(options.secret ?? process.env.MINIGAME_SECRET ?? '');
  const platformEnabled = options.platformEnabled ?? Boolean(secret);
  const wallet = options.wallet ?? new PlatformWallet(options.walletOptions);
  const socketConfiguration = {
    serveClient: true,
    transports: ['websocket', 'polling'],
    maxHttpBufferSize: 16 * 1024,
    path: `${basePath}/socket.io`,
    ...options.socketOptions,
  };
  if (options.corsOrigin === undefined && !socketConfiguration.allowRequest) {
    socketConfiguration.allowRequest = allowSameOriginRequest;
  }
  if (options.corsOrigin !== undefined) {
    socketConfiguration.cors = {
      origin: options.corsOrigin,
      methods: ['GET', 'POST'],
    };
  }
  const io = new SocketIOServer(httpServer, socketConfiguration);
  const engine = options.engine ?? new GameEngine({
    maxRooms: Number(process.env.MAX_ROOMS) || undefined,
    rtp: process.env.TARGET_RTP === undefined ? undefined : Number(process.env.TARGET_RTP),
    ...options.engineOptions,
  });
  const startedAt = Date.now();
  const timers = new Set();

  app.disable('x-powered-by');
  app.use(express.json({ limit: '16kb' }));
  const healthPaths = basePath ? ['/health', `${basePath}/health`] : ['/health'];
  app.get(healthPaths, (_request, response) => {
    response.json({
      ok: true,
      status: 'ok',
      uptimeSeconds: Math.floor((Date.now() - startedAt) / 1000),
      entryMode: 'match',
      tableCount: TABLE_COUNT,
      ...engine.stats(),
    });
  });

  const publicDirectory = options.staticDir === undefined ? defaultPublicDirectory : options.staticDir;
  if (publicDirectory) {
    // 同时挂根路径与子路径，直连 18082 和经反代访问都能工作
    app.use(express.static(publicDirectory));
    if (basePath) app.use(basePath, express.static(publicDirectory));
  }

  function emitSnapshot(room, advanceSequence = true) {
    if (!room || room.playerCount === 0) return;
    io.to(socketRoom(room.id)).emit('game:snapshot', room.snapshot(advanceSequence));
  }

  function emitNotice(roomId, notice) {
    io.to(socketRoom(roomId)).emit('room:notice', {
      roomId,
      serverTime: Date.now(),
      ...notice,
    });
  }

  function detachSocket(socket, reason = 'left') {
    const { roomId, playerId } = socket.data;
    if (!roomId || !playerId) return null;
    socket.data.roomId = null;
    socket.data.playerId = null;
    socket.data.resumeToken = null;
    const player = engine.leave(roomId, playerId);
    if (!player) return null;
    emitNotice(roomId, {
      type: 'player-left',
      playerId,
      name: player.name,
      reason,
      message: `${player.name} 离开了海域`,
    });
    emitSnapshot(engine.getRoom(roomId));
    return player;
  }

  io.on('connection', (socket) => {
    socket.data.roomId = null;
    socket.data.playerId = null;
    socket.data.resumeToken = null;
    socket.data.joining = false;
    socket.data.lastJoinAt = Number.NEGATIVE_INFINITY;
    socket.data.lastAimAt = Number.NEGATIVE_INFINITY;
    socket.data.lastPowerAt = Number.NEGATIVE_INFINITY;

    socket.on('room:join', async (payload = {}, ack) => {
      const reply = safeAck(ack);
      const joinNow = Date.now();
      if (joinNow - socket.data.lastJoinAt < 400) {
        reply({
          ok: false,
          roomId: payload?.roomId ?? null,
          playerId: null,
          resumeToken: null,
          state: null,
          error: { code: 'RATE_LIMITED', message: '加入房间过快，请稍后重试' },
        });
        return;
      }
      socket.data.lastJoinAt = joinNow;
      if (socket.data.joining) {
        reply({
          ok: false,
          roomId: payload?.roomId ?? null,
          playerId: null,
          resumeToken: null,
          state: null,
          error: { code: 'JOIN_IN_PROGRESS', message: '正在加入房间，请稍候' },
        });
        return;
      }
      socket.data.joining = true;

      try {
        let joinPayload = payload && typeof payload === 'object' ? { ...payload } : {};
        if (platformEnabled) {
          const identity = verifyLaunchTicket(joinPayload, secret);
          const walletState = await wallet.balance(identity.uid);
          joinPayload = {
            ...joinPayload,
            ...identity,
            score: walletState.balance
          };
        }
        if (socket.data.roomId && socket.data.roomId === joinPayload?.roomId) {
          const room = engine.getRoom(socket.data.roomId);
          const player = room?.getPlayer(socket.data.playerId);
          if (room && player) {
            reply({
              ok: true,
              roomId: room.id,
              playerId: player.id,
              seat: player.seat,
              side: SEATS[player.seat]?.side ?? null,
              position: SEATS[player.seat]?.position ?? null,
              resumeToken: player.resumeToken,
              state: room.snapshot(false),
              resumed: true,
            });
            return;
          }
        }

        const result = engine.join(joinPayload);
        if (!result.ok) {
          reply(result);
          return;
        }

        const previousRoomId = socket.data.roomId;
        if (previousRoomId) {
          const previousPlayerId = socket.data.playerId;
          const previous = detachSocket(socket, 'switched-room');
          await socket.leave(socketRoom(previousRoomId));
          if (previous) {
            // detachSocket already emitted the notice; keep these values only for clarity in adapters.
            void previousPlayerId;
          }
        }

        await socket.join(socketRoom(result.roomId));
        socket.data.roomId = result.roomId;
        socket.data.playerId = result.playerId;
        socket.data.resumeToken = result.resumeToken;
        socket.data.uid = platformEnabled ? Number(joinPayload.uid) : null;
        reply(result);

        const room = engine.getRoom(result.roomId);
        const player = room?.getPlayer(result.playerId);
        emitNotice(result.roomId, {
          type: result.resumed ? 'player-resumed' : 'player-joined',
          playerId: result.playerId,
          name: player?.name ?? '游客',
          seat: player?.seat ?? null,
          side: player ? SEATS[player.seat]?.side ?? null : null,
          position: player ? SEATS[player.seat]?.position ?? null : null,
          message: result.resumed
            ? `${player?.name ?? '游客'} 已重新连接`
            : `${player?.name ?? '游客'} 加入了海域`,
        });
        emitSnapshot(room);
      } catch (error) {
        reply({
          ok: false,
          roomId: payload?.roomId ?? null,
          playerId: null,
          resumeToken: null,
          state: null,
          error: { code: 'JOIN_FAILED', message: error?.message || '暂时无法加入房间' },
        });
        if (options.logger?.error) options.logger.error(error);
      } finally {
        socket.data.joining = false;
      }
    });

    socket.on('player:aim', (payload = {}) => {
      const aimNow = Date.now();
      if (aimNow - socket.data.lastAimAt < 20) return;
      socket.data.lastAimAt = aimNow;
      const room = engine.getRoom(socket.data.roomId);
      if (!room || !socket.data.playerId) return;
      room.setAim(socket.data.playerId, payload?.angle);
    });

    socket.on('player:power', (payload = {}) => {
      const powerNow = Date.now();
      if (powerNow - socket.data.lastPowerAt < 60) return;
      socket.data.lastPowerAt = powerNow;
      const room = engine.getRoom(socket.data.roomId);
      if (!room || !socket.data.playerId) return;
      room.setPower(socket.data.playerId, payload?.power ?? payload?.bet);
    });

    // New clients call this event; player:power remains supported for older builds.
    socket.on('player:bet', (payload = {}) => {
      const powerNow = Date.now();
      if (powerNow - socket.data.lastPowerAt < 60) return;
      socket.data.lastPowerAt = powerNow;
      const room = engine.getRoom(socket.data.roomId);
      if (!room || !socket.data.playerId) return;
      room.setBet(socket.data.playerId, payload?.bet ?? payload?.power);
    });

    socket.on('player:fire', async (payload = {}, ack) => {
      const reply = safeAck(ack);
      const room = engine.getRoom(socket.data.roomId);
      if (!room || !socket.data.playerId) {
        reply(connectionError(payload?.commandId ?? null));
        return;
      }
      const command = payload && typeof payload === 'object' ? payload : {};
      const result = room.fire(socket.data.playerId, command, Date.now(), { deferWallet: platformEnabled });
      if (!result.ok || !platformEnabled) {
        reply(result);
        return;
      }
      const player = room.getPlayer(socket.data.playerId);
      try {
        const walletState = await wallet.adjust({
          order_no: `fish:shot:${result.shotId}`,
          uid: player.uid,
          game_code: 'deepsea_hunter',
          table_no: Number(String(room.id).replace(/\D/g, '')),
          round_no: room.id,
          reason: 'cannon_shot',
          amount: -result.bet
        });
        room.commitFire(player.id, result.commandId, walletState.balance);
        result.score = walletState.balance;
        reply(result);
      } catch (error) {
        room.rollbackFire(player.id, result.commandId);
        reply({
          ok: false,
          commandId: result.commandId,
          error: { code: 'WALLET_REJECTED', message: error?.message || '平台钱包扣款失败' }
        });
      }
    });

    socket.on('disconnect', (reason) => {
      detachSocket(socket, reason || 'disconnected');
    });
  });

  const payoutRetries = new Map();
  async function settleResolution(room, event) {
    if (!event.captured || !platformEnabled) {
      io.to(socketRoom(room.id)).emit('shot:resolved', event);
      if (event.captured) io.to(socketRoom(room.id)).emit('game:catch', event);
      return;
    }
    const player = room.getPlayer(event.playerId);
    if (!player?.uid) return;
    try {
      const walletState = await wallet.adjust({
        order_no: `fish:catch:${event.resolutionId}`,
        uid: player.uid,
        game_code: 'deepsea_hunter',
        table_no: Number(String(room.id).replace(/\D/g, '')),
        round_no: room.id,
        reason: 'fish_capture',
        amount: event.reward
      });
      payoutRetries.delete(event.resolutionId);
      player.score = walletState.balance;
      event.score = walletState.balance;
      io.to(socketRoom(room.id)).emit('shot:resolved', event);
      io.to(socketRoom(room.id)).emit('game:catch', event);
    } catch {
      const attempts = (payoutRetries.get(event.resolutionId) || 0) + 1;
      payoutRetries.set(event.resolutionId, attempts);
      const retry = setTimeout(() => void settleResolution(room, event), Math.min(30_000, attempts * 1000));
      retry.unref?.();
    }
  }

  function startLoops() {
    if (timers.size > 0) return;
    const simulationInterval = 1000 / SIMULATION_HZ;
    let previousTick = Date.now();
    const simulationTimer = setInterval(() => {
      const now = Date.now();
      const elapsedSeconds = Math.min((now - previousTick) / 1000, 0.1);
      previousTick = now;
      engine.tick(elapsedSeconds);
      for (const room of engine.rooms.values()) {
        for (const event of room.drainEvents()) {
          void settleResolution(room, event);
        }
      }
    }, simulationInterval);
    simulationTimer.unref?.();
    timers.add(simulationTimer);

    const snapshotTimer = setInterval(() => {
      for (const room of engine.rooms.values()) emitSnapshot(room);
    }, SNAPSHOT_INTERVAL_MS);
    snapshotTimer.unref?.();
    timers.add(snapshotTimer);
  }

  function stopLoops() {
    for (const timer of timers) clearInterval(timer);
    timers.clear();
  }

  httpServer.on('listening', startLoops);
  httpServer.on('close', stopLoops);

  async function start(
    port = options.port ?? (Number(process.env.PORT) || 3000),
    host = options.host ?? process.env.HOST ?? '0.0.0.0',
  ) {
    if (httpServer.listening) return httpServer.address();
    await new Promise((resolve, reject) => {
      const onError = (error) => {
        httpServer.off('listening', onListening);
        reject(error);
      };
      const onListening = () => {
        httpServer.off('error', onError);
        resolve();
      };
      httpServer.once('error', onError);
      httpServer.once('listening', onListening);
      httpServer.listen(port, host);
    });
    return httpServer.address();
  }

  async function stop() {
    stopLoops();
    if (!httpServer.listening) {
      io.close();
      return;
    }
    await new Promise((resolve) => io.close(resolve));
  }

  return {
    app,
    httpServer,
    io,
    engine,
    start,
    stop,
    close: stop,
  };
}

const entryFile = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])).href : '';
if (entryFile === import.meta.url) {
  const gameServer = createGameServer();
  gameServer.start().then((address) => {
    const printableAddress = typeof address === 'string' ? address : `${address.address}:${address.port}`;
    console.log(`Fishing game listening on http://${printableAddress}`);
  }).catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
