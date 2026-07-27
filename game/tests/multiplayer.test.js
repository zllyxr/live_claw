import assert from 'node:assert/strict';
import test from 'node:test';
import { createHmac } from 'node:crypto';
import { io as createSocketClient } from 'socket.io-client';
import {
  BET_LEVELS,
  DEFAULT_RTP,
  GameEngine,
  GameRoom,
  SEATS,
  STARTING_SCORE,
} from '../src/engine.js';
import { captureProbability } from '../src/capture-policy.js';
import { createGameServer } from '../src/server.js';

const EVENT_TIMEOUT_MS = 3_000;

function waitForEvent(socket, eventName, predicate = () => true, timeoutMs = EVENT_TIMEOUT_MS) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error(`Timed out waiting for ${eventName}`));
    }, timeoutMs);

    const onEvent = (...args) => {
      let matches = false;
      try {
        matches = predicate(...args);
      } catch (error) {
        cleanup();
        reject(error);
        return;
      }
      if (!matches) return;
      cleanup();
      resolve(args.length <= 1 ? args[0] : args);
    };

    const onConnectError = (error) => {
      cleanup();
      reject(error);
    };

    const onUnexpectedDisconnect = (reason) => {
      cleanup();
      reject(new Error(`Socket disconnected while waiting for ${eventName}: ${reason}`));
    };

    function cleanup() {
      clearTimeout(timeout);
      socket.off(eventName, onEvent);
      socket.off('connect_error', onConnectError);
      if (eventName !== 'disconnect') socket.off('disconnect', onUnexpectedDisconnect);
    }

    socket.on(eventName, onEvent);
    socket.on('connect_error', onConnectError);
    if (eventName !== 'disconnect') socket.on('disconnect', onUnexpectedDisconnect);
  });
}

function emitWithAck(socket, eventName, payload, timeoutMs = EVENT_TIMEOUT_MS) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error(`Timed out waiting for ${eventName} acknowledgement`));
    }, timeoutMs);

    socket.emit(eventName, payload, (response) => {
      clearTimeout(timeout);
      resolve(response);
    });
  });
}

async function waitForCondition(predicate, message, timeoutMs = EVENT_TIMEOUT_MS) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error(message);
}

async function createFixture(t, engineOptions = {}) {
  // Keeping all generated fish well away from the two vertical test shots makes
  // bullet assertions deterministic while the normal simulation loop is running.
  const engine = new GameEngine({ random: () => 0, ...engineOptions });
  const gameServer = createGameServer({ engine, staticDir: false });
  const sockets = new Set();
  const address = await gameServer.start(0, '127.0.0.1');
  const url = `http://127.0.0.1:${address.port}`;

  t.after(async () => {
    for (const socket of sockets) {
      socket.removeAllListeners();
      socket.disconnect();
    }
    sockets.clear();
    await gameServer.stop();
  });

  async function connect() {
    const socket = createSocketClient(url, {
      autoConnect: false,
      forceNew: true,
      reconnection: false,
      transports: ['websocket'],
    });
    sockets.add(socket);
    const connected = waitForEvent(socket, 'connect');
    socket.connect();
    await connected;
    return socket;
  }

  return { engine, gameServer, sockets, url, connect };
}

async function joinRoom(socket, roomId, name, extra = {}) {
  const result = await emitWithAck(socket, 'room:join', { roomId, name, ...extra });
  assert.equal(result.ok, true, result.error?.message);
  assert.equal(result.roomId, roomId);
  assert.ok(result.playerId);
  assert.ok(result.resumeToken);
  return result;
}

function playerById(snapshot, playerId) {
  return snapshot.players.find((player) => player.id === playerId);
}

function installStationaryTarget(room, overrides = {}) {
  const target = {
    id: 'thin-target',
    type: 'puffer',
    assetKey: 'puffer',
    tier: 'common',
    multiplier: 5,
    x: 430,
    y: 630,
    baseY: 630,
    vx: 0,
    angle: 0,
    scale: 1,
    radius: 4,
    age: 0,
    waveAmplitude: 0,
    waveSpeed: 0,
    phase: 0,
    ...overrides,
  };
  room.fishes.set(target.id, target);
  return target;
}

test('effective hit uses the probability roll and pays bet times multiplier on capture', () => {
  let id = 0;
  const room = new GameRoom('collision-room', {
    random: () => 0,
    captureRng: () => 0,
    now: () => 1_000,
    idFactory: () => `entity-${++id}`,
  });
  room.fishes.clear();
  const joined = room.addPlayer({ name: '概率玩家', bet: 2 });
  assert.equal(joined.ok, true);
  installStationaryTarget(room);

  const fired = room.fire(joined.player.id, { commandId: 'swept-shot', angle: -Math.PI / 2 });
  assert.equal(fired.ok, true);
  room.tick(0.05);

  assert.equal(room.fishes.has('thin-target'), false);
  assert.equal(room.bullets.size, 0);
  assert.equal(room.getPlayer(joined.player.id).score, STARTING_SCORE - 2 + 10);
  const events = room.drainEvents();
  assert.equal(events.length, 1);
  assert.equal(events[0].fishId, 'thin-target');
  assert.equal(events[0].playerId, joined.player.id);
  assert.equal(events[0].captured, true);
  assert.equal(events[0].multiplier, 5);
  assert.equal(events[0].bet, 2);
  assert.equal(events[0].reward, 10);
  assert.deepEqual(room.drainEvents(), []);
});

test('effective hit can escape without HP or deterministic damage', () => {
  let id = 0;
  const room = new GameRoom('escape-room', {
    random: () => 0,
    captureRng: () => 1,
    now: () => 1_000,
    idFactory: () => `escape-${++id}`,
  });
  room.fishes.clear();
  const joined = room.addPlayer({ name: '逃脱见证者', bet: 2 });
  installStationaryTarget(room);

  const fired = room.fire(joined.player.id, {
    commandId: 'forced-escape',
    angle: -Math.PI / 2,
    captured: true,
    payout: 999_999,
    bet: 50,
  });
  assert.equal(fired.ok, true);
  room.tick(0.05);

  assert.equal(room.fishes.has('thin-target'), true);
  assert.equal(room.bullets.size, 0);
  assert.equal(room.getPlayer(joined.player.id).score, STARTING_SCORE - 2);
  const [resolution] = room.drainEvents();
  assert.equal(resolution.captured, false);
  assert.equal(resolution.reward, 0);
  assert.equal(resolution.bet, 2);
});

test('capture table and four cabinet seats match the arcade model', () => {
  assert.deepEqual(BET_LEVELS, [1, 2, 5, 10, 20, 50]);
  assert.equal(DEFAULT_RTP, 0.72);
  assert.equal(captureProbability(2, DEFAULT_RTP), 0.36);
  assert.equal(captureProbability(20, DEFAULT_RTP), 0.036);
  assert.equal(captureProbability(80, DEFAULT_RTP), 0.009);
  assert.deepEqual(SEATS.map((seat) => seat.side), ['south', 'south', 'north', 'north']);
  assert.deepEqual(
    SEATS.map((seat) => seat.position),
    ['south-left', 'south-right', 'north-left', 'north-right'],
  );
  assert.deepEqual(
    SEATS.map(({ x, y, facing }) => ({ x, y, facing })),
    [
      { x: 430, y: 690, facing: -Math.PI / 2 },
      { x: 850, y: 690, facing: -Math.PI / 2 },
      { x: 430, y: 30, facing: Math.PI / 2 },
      { x: 850, y: 30, facing: Math.PI / 2 },
    ],
  );
  assert.equal(new Set(SEATS.map((seat) => `${seat.x}:${seat.y}`)).size, 4);
  for (const seat of SEATS) {
    const towardCenter = (640 - seat.x) * Math.cos(seat.facing) + (360 - seat.y) * Math.sin(seat.facing);
    assert.ok(towardCenter > 0, `${seat.position} cannon must face the shared arena`);
  }
});

test('server spawns a coherent fish school without changing capture authority', () => {
  let id = 0;
  const randomValues = [0.1, 0.9, 0.5, 0.4, 0.3, 0.2, 0.6];
  const room = new GameRoom('school-room', {
    random: () => randomValues.shift() ?? 0.5,
    idFactory: () => `school-entity-${++id}`,
  });
  room.fishes.clear();

  const school = room.spawnSchool(5, false);
  assert.equal(school.length, 5);
  assert.equal(new Set(school.map((fish) => fish.schoolId)).size, 1);
  assert.equal(new Set(school.map((fish) => fish.type)).size, 1);
  assert.equal(new Set(school.map((fish) => fish.vx)).size, 1);
  assert.equal(new Set(school.map((fish) => fish.waveSpeed)).size, 1);
  assert.equal(new Set(school.map((fish) => fish.phase)).size, 1);
  assert.deepEqual(
    school.map((fish) => Math.round(fish.baseY - school[0].baseY)),
    [0, -36, 36, -68, 68],
  );

  const snapshot = room.snapshot(false);
  assert.ok(snapshot.fishes.every((fish) => fish.schoolId === school[0].schoolId));
});

test('four simultaneous successful rolls can award one shared fish at most once', () => {
  let id = 0;
  const room = new GameRoom('atomic-capture', {
    random: () => 0,
    captureRng: () => 0,
    now: () => 2_000,
    idFactory: () => `atomic-${++id}`,
  });
  room.fishes.clear();
  room.spawnFish = () => null;
  installStationaryTarget(room, { x: 640, y: 360, baseY: 360, radius: 45, multiplier: 20 });

  const players = SEATS.map((seat, index) => {
    const result = room.addPlayer({ name: `座位${index + 1}`, seat: index, bet: 2 });
    assert.equal(result.ok, true);
    const angle = Math.atan2(360 - seat.y, 640 - seat.x);
    const fired = room.fire(result.player.id, { commandId: `same-fish-${index}`, angle });
    assert.equal(fired.ok, true);
    return result.player;
  });

  for (let step = 0; step < 10 && room.fishes.has('thin-target'); step += 1) room.tick(0.1);
  const captures = room.drainEvents().filter((event) => event.captured && event.fishId === 'thin-target');
  assert.equal(captures.length, 1);
  assert.equal(room.fishes.has('thin-target'), false);
  assert.equal(
    players.reduce((total, player) => total + room.getPlayer(player.id).score, 0),
    STARTING_SCORE * 4 - 8 + 40,
  );
});

test('engine caps active rooms before allocating another simulation', () => {
  const engine = new GameEngine({ random: () => 0, maxRooms: 1 });
  const first = engine.join({ roomId: 'only-room', name: '甲' });
  const rejected = engine.join({ roomId: 'extra-room', name: '乙' });

  assert.equal(first.ok, true);
  assert.equal(rejected.ok, false);
  assert.equal(rejected.error.code, 'SERVER_FULL');
  assert.equal(engine.rooms.size, 1);
  assert.equal(engine.getRoom('extra-room'), null);
});

test('GET /health reports a live, empty game server', async (t) => {
  const { url } = await createFixture(t);

  const response = await fetch(`${url}/health`);
  assert.equal(response.status, 200);
  assert.match(response.headers.get('content-type') ?? '', /^application\/json\b/);
  const health = await response.json();
  assert.equal(health.ok, true);
  assert.equal(health.status, 'ok');
  assert.equal(health.rooms, 0);
  assert.equal(health.players, 0);
  assert.equal(health.bullets, 0);
  assert.equal(health.targetRtp, DEFAULT_RTP);
  assert.equal(typeof health.uptimeSeconds, 'number');
});

test('websocket handshake rejects an unexpected browser origin', async (t) => {
  const { url, sockets } = await createFixture(t);
  const socket = createSocketClient(url, {
    autoConnect: false,
    forceNew: true,
    reconnection: false,
    transports: ['websocket'],
    extraHeaders: { Origin: 'https://unexpected.example' },
  });
  sockets.add(socket);

  const error = await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('cross-origin handshake was not rejected')), EVENT_TIMEOUT_MS);
    socket.once('connect', () => {
      clearTimeout(timeout);
      reject(new Error('cross-origin websocket unexpectedly connected'));
    });
    socket.once('connect_error', (connectionError) => {
      clearTimeout(timeout);
      resolve(connectionError);
    });
    socket.connect();
  });

  assert.ok(error instanceof Error);
  assert.equal(socket.connected, false);
});

test('two independent clients joining one room both receive both players', async (t) => {
  const { connect } = await createFixture(t);
  const first = await connect();
  const second = await connect();
  const firstJoin = await joinRoom(first, 'shared-room', '甲');

  const firstSeesBoth = waitForEvent(
    first,
    'game:snapshot',
    (snapshot) => snapshot.roomId === 'shared-room' && snapshot.players.length === 2,
  );
  const secondSeesBoth = waitForEvent(
    second,
    'game:snapshot',
    (snapshot) => snapshot.roomId === 'shared-room' && snapshot.players.length === 2,
  );
  const secondJoin = await joinRoom(second, 'shared-room', '乙');
  const [firstSnapshot, secondSnapshot] = await Promise.all([firstSeesBoth, secondSeesBoth]);

  const expectedPlayers = new Set([firstJoin.playerId, secondJoin.playerId]);
  assert.deepEqual(new Set(firstSnapshot.players.map((player) => player.id)), expectedPlayers);
  assert.deepEqual(new Set(secondSnapshot.players.map((player) => player.id)), expectedPlayers);
});

test('two players can fire at the same time and both observe both bullets and score deductions', async (t) => {
  const { connect } = await createFixture(t);
  const first = await connect();
  const second = await connect();
  const firstJoin = await joinRoom(first, 'crossfire', '甲');
  const secondJoin = await joinRoom(second, 'crossfire', '乙');
  const playerIds = [firstJoin.playerId, secondJoin.playerId];

  const containsBothShots = (snapshot) => {
    if (snapshot.roomId !== 'crossfire') return false;
    const owners = new Set(snapshot.bullets.map((bullet) => bullet.ownerId));
    return playerIds.every((playerId) => owners.has(playerId));
  };
  const firstSnapshotPromise = waitForEvent(first, 'game:snapshot', containsBothShots);
  const secondSnapshotPromise = waitForEvent(second, 'game:snapshot', containsBothShots);

  const [firstFire, secondFire] = await Promise.all([
    emitWithAck(first, 'player:fire', { commandId: 'shot-first', angle: -Math.PI / 2 }),
    emitWithAck(second, 'player:fire', { commandId: 'shot-second', angle: -Math.PI / 2 }),
  ]);
  assert.equal(firstFire.ok, true);
  assert.equal(secondFire.ok, true);
  assert.equal(firstFire.score, STARTING_SCORE - 2);
  assert.equal(secondFire.score, STARTING_SCORE - 2);

  const [firstSnapshot, secondSnapshot] = await Promise.all([
    firstSnapshotPromise,
    secondSnapshotPromise,
  ]);
  for (const snapshot of [firstSnapshot, secondSnapshot]) {
    assert.equal(playerById(snapshot, firstJoin.playerId).score, STARTING_SCORE - 2);
    assert.equal(playerById(snapshot, secondJoin.playerId).score, STARTING_SCORE - 2);
    assert.ok(snapshot.bullets.some((bullet) => bullet.id === firstFire.bulletId));
    assert.ok(snapshot.bullets.some((bullet) => bullet.id === secondFire.bulletId));
  }
});

test('retrying the same commandId creates one bullet and deducts score once', async (t) => {
  const { connect, engine } = await createFixture(t);
  const client = await connect();
  const joined = await joinRoom(client, 'idempotent', '重试玩家');
  const command = { commandId: 'stable-command-id', angle: -Math.PI / 2 };

  const firstAck = await emitWithAck(client, 'player:fire', command);
  const retryAck = await emitWithAck(client, 'player:fire', command);

  assert.deepEqual(retryAck, firstAck);
  assert.equal(firstAck.ok, true);
  assert.equal(firstAck.score, STARTING_SCORE - 2);
  const room = engine.getRoom('idempotent');
  assert.equal(room.getPlayer(joined.playerId).score, STARTING_SCORE - 2);
  assert.equal(room.bullets.size, 1);
  assert.deepEqual([...room.bullets.keys()], [firstAck.bulletId]);
});

test('snapshots and projectiles remain isolated between rooms', async (t) => {
  const { connect, engine } = await createFixture(t);
  const alpha = await connect();
  const beta = await connect();
  const alphaJoin = await joinRoom(alpha, 'alpha-room', 'Alpha');
  const betaJoin = await joinRoom(beta, 'beta-room', 'Beta');

  const alphaShotSnapshot = waitForEvent(
    alpha,
    'game:snapshot',
    (snapshot) => snapshot.roomId === 'alpha-room' && snapshot.bullets.length === 1,
  );
  const nextBetaSnapshot = waitForEvent(
    beta,
    'game:snapshot',
    (snapshot) => snapshot.roomId === 'beta-room',
  );
  const fireAck = await emitWithAck(alpha, 'player:fire', {
    commandId: 'alpha-only-shot',
    angle: -Math.PI / 2,
  });
  assert.equal(fireAck.ok, true);

  const [alphaSnapshot, betaSnapshot] = await Promise.all([
    alphaShotSnapshot,
    nextBetaSnapshot,
  ]);
  assert.deepEqual(alphaSnapshot.players.map((player) => player.id), [alphaJoin.playerId]);
  assert.ok(alphaSnapshot.bullets.some((bullet) => bullet.id === fireAck.bulletId));
  assert.deepEqual(betaSnapshot.players.map((player) => player.id), [betaJoin.playerId]);
  assert.equal(betaSnapshot.bullets.length, 0);
  assert.equal(engine.getRoom('alpha-room').bullets.size, 1);
  assert.equal(engine.getRoom('beta-room').bullets.size, 0);
});

test('a fifth player is rejected when a room already has four players', async (t) => {
  const { connect, engine } = await createFixture(t);
  const clients = await Promise.all(Array.from({ length: 5 }, () => connect()));

  const accepted = [];
  for (let index = 0; index < 4; index += 1) {
    accepted.push(await joinRoom(clients[index], 'capacity', `玩家${index + 1}`));
  }
  const rejected = await emitWithAck(clients[4], 'room:join', {
    roomId: 'capacity',
    name: '玩家5',
  });

  assert.equal(rejected.ok, false);
  assert.equal(rejected.error.code, 'ROOM_FULL');
  assert.equal(rejected.playerId, null);
  assert.equal(rejected.resumeToken, null);
  assert.equal(engine.getRoom('capacity').playerCount, 4);
  assert.equal(new Set(accepted.map((result) => result.playerId)).size, 4);
});

test('resumeToken reconnects within TTL with the same player, seat, and score', async (t) => {
  const { connect, engine } = await createFixture(t, { resumeTtlMs: 2_000 });
  const originalSocket = await connect();
  const originalJoin = await joinRoom(originalSocket, 'resume-room', '潜水员');
  const originalPlayer = playerById(originalJoin.state, originalJoin.playerId);
  const fireAck = await emitWithAck(originalSocket, 'player:fire', {
    commandId: 'before-disconnect',
    angle: -Math.PI / 2,
  });
  assert.equal(fireAck.ok, true);

  originalSocket.disconnect();
  await waitForCondition(
    () => engine.resumeSessions.has(originalJoin.resumeToken),
    'server did not record the resumable session',
  );

  const resumedSocket = await connect();
  const resumed = await joinRoom(resumedSocket, 'resume-room', '潜水员', {
    resumeToken: originalJoin.resumeToken,
  });
  const resumedPlayer = playerById(resumed.state, resumed.playerId);

  assert.equal(resumed.resumed, true);
  assert.equal(resumed.playerId, originalJoin.playerId);
  assert.equal(resumed.resumeToken, originalJoin.resumeToken);
  assert.equal(resumedPlayer.id, originalPlayer.id);
  assert.equal(resumedPlayer.seat, originalPlayer.seat);
  assert.equal(resumedPlayer.score, fireAck.score);

  const retriedAfterReconnect = await emitWithAck(resumedSocket, 'player:fire', {
    commandId: 'before-disconnect',
    angle: -Math.PI / 2,
  });
  assert.deepEqual(retriedAfterReconnect, fireAck);
  assert.equal(engine.getRoom('resume-room').getPlayer(resumed.playerId).score, fireAck.score);
});

test('platform ticket forces the assigned 1..1000 table and every shot debits the user wallet', async (t) => {
  const secret = 'test-minigame-secret';
  const uid = 918;
  const table = 9;
  const ts = Math.floor(Date.now() / 1000);
  const sig = createHmac('sha256', secret)
    .update(`deepsea_hunter|${uid}|${ts}`)
    .digest('hex')
    .slice(0, 32);
  let balance = 777;
  const orders = [];
  const wallet = {
    async balance(requestUID) {
      assert.equal(requestUID, uid);
      return { ok: true, balance };
    },
    async adjust(order) {
      orders.push(order);
      balance += order.amount;
      return { ok: true, balance };
    },
  };
  const engine = new GameEngine({ random: () => 0 });
  const server = createGameServer({
    engine,
    staticDir: false,
    platformEnabled: true,
    secret,
    wallet,
  });
  const address = await server.start(0, '127.0.0.1');
  const socket = createSocketClient(`http://127.0.0.1:${address.port}`, {
    transports: ['websocket'],
    forceNew: true,
  });
  t.after(async () => {
    socket.disconnect();
    await server.stop();
  });
  await waitForEvent(socket, 'connect');

  const joined = await emitWithAck(socket, 'room:join', {
    roomId: 'MANUAL-ROOM-MUST-BE-IGNORED',
    uid,
    name: '平台玩家',
    table,
    ts,
    sig,
  });
  assert.equal(joined.ok, true);
  assert.equal(joined.roomId, 'T0009');
  assert.equal(playerById(joined.state, joined.playerId).score, 777);

  const fired = await emitWithAck(socket, 'player:fire', {
    commandId: 'wallet-shot-1',
    angle: -Math.PI / 2,
  });
  assert.equal(fired.ok, true);
  assert.equal(fired.score, 775);
  assert.equal(orders.length, 1);
  assert.equal(orders[0].uid, uid);
  assert.equal(orders[0].table_no, table);
  assert.equal(orders[0].amount, -2);
  assert.equal(orders[0].reason, 'cannon_shot');
});
