import { randomUUID } from 'node:crypto';
import {
  BET_LEVELS,
  DEFAULT_RTP,
  cryptoCaptureRng,
  normaliseBet,
  normaliseRtp,
  resolveCapture,
} from './capture-policy.js';

export const WORLD_WIDTH = 1280;
export const WORLD_HEIGHT = 720;
export const MAX_PLAYERS = 4;
export const SIMULATION_HZ = 20;
export const SNAPSHOT_INTERVAL_MS = 100;
export const MIN_POWER = BET_LEVELS[0];
export const MAX_POWER = BET_LEVELS[BET_LEVELS.length - 1];
export const STARTING_SCORE = 10_000;
export { BET_LEVELS, DEFAULT_RTP };

export const SEATS = Object.freeze([
  Object.freeze({ seat: 0, side: 'south', position: 'south-left', x: 430, y: 690, facing: -Math.PI / 2, minAimOffset: -Math.PI * 0.44, maxAimOffset: Math.PI * 0.44 }),
  Object.freeze({ seat: 1, side: 'south', position: 'south-right', x: 850, y: 690, facing: -Math.PI / 2, minAimOffset: -Math.PI * 0.44, maxAimOffset: Math.PI * 0.44 }),
  Object.freeze({ seat: 2, side: 'north', position: 'north-left', x: 430, y: 30, facing: Math.PI / 2, minAimOffset: -Math.PI * 0.44, maxAimOffset: Math.PI * 0.44 }),
  Object.freeze({ seat: 3, side: 'north', position: 'north-right', x: 850, y: 30, facing: Math.PI / 2, minAimOffset: -Math.PI * 0.44, maxAimOffset: Math.PI * 0.44 }),
]);

const PLAYER_COLORS = Object.freeze(['#56d7ff', '#ffd45c', '#ff79a9', '#80ed99']);
const FIRE_INTERVAL_MS = 120;
const BULLET_SPEED = 820;
const BULLET_LIFETIME_SECONDS = 2.25;
const COMMAND_CACHE_SIZE = 192;
const RESUME_TTL_MS = 15_000;
const EMPTY_ROOM_TTL_MS = 15_000;
const DEFAULT_MAX_ROOMS = 1000;

// These are gameplay species, not colour variants. assetKey is the stable bridge
// to the browser's licensed creature art and animation catalog.
export const FISH_TYPES = Object.freeze({
  tuna: Object.freeze({ multiplier: 2, tier: 'common', assetKey: 'tuna', speed: 132, radius: 19, scale: 0.7, weight: 24 }),
  lionfish: Object.freeze({ multiplier: 3, tier: 'common', assetKey: 'lionfish', speed: 105, radius: 23, scale: 0.8, weight: 20 }),
  puffer: Object.freeze({ multiplier: 5, tier: 'common', assetKey: 'puffer', speed: 90, radius: 27, scale: 0.9, weight: 17 }),
  grouper: Object.freeze({ multiplier: 8, tier: 'common', assetKey: 'grouper', speed: 82, radius: 33, scale: 1.04, weight: 14 }),
  turtle: Object.freeze({ multiplier: 12, tier: 'common', assetKey: 'turtle', speed: 66, radius: 40, scale: 1.18, weight: 10 }),
  manta: Object.freeze({ multiplier: 20, tier: 'common', assetKey: 'manta', speed: 60, radius: 49, scale: 1.38, weight: 7 }),
  hammerhead: Object.freeze({ multiplier: 30, tier: 'boss', assetKey: 'hammerhead', speed: 70, radius: 59, scale: 1.62, weight: 4 }),
  octopus: Object.freeze({ multiplier: 40, tier: 'boss', assetKey: 'octopus', speed: 46, radius: 57, scale: 1.58, weight: 3 }),
  orca: Object.freeze({ multiplier: 60, tier: 'boss', assetKey: 'orca', speed: 58, radius: 70, scale: 1.9, weight: 2 }),
  anglerfish: Object.freeze({ multiplier: 80, tier: 'boss', assetKey: 'anglerfish', speed: 42, radius: 65, scale: 1.78, weight: 2 }),
});

const FISH_WEIGHT_TOTAL = Object.values(FISH_TYPES).reduce((sum, fish) => sum + fish.weight, 0);
const SCHOOL_FISH_TYPES = Object.freeze(['tuna', 'lionfish', 'puffer', 'grouper']);
const SCHOOL_OFFSETS = Object.freeze([
  Object.freeze({ behind: 0, y: 0 }),
  Object.freeze({ behind: 62, y: -36 }),
  Object.freeze({ behind: 62, y: 36 }),
  Object.freeze({ behind: 124, y: -68 }),
  Object.freeze({ behind: 124, y: 68 }),
]);

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(maximum, value));
}

function finiteNumber(value) {
  const number = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(number) ? number : null;
}

function normaliseAngle(value) {
  const number = finiteNumber(value);
  if (number === null) return null;
  return Math.atan2(Math.sin(number), Math.cos(number));
}

function clampAimToSeat(seatNumber, angle) {
  const seat = SEATS[seatNumber];
  const normalised = normaliseAngle(angle);
  if (!seat || normalised === null) return null;
  const relative = Math.atan2(Math.sin(normalised - seat.facing), Math.cos(normalised - seat.facing));
  return normaliseAngle(seat.facing + clamp(relative, seat.minAimOffset, seat.maxAimOffset));
}

function round(value, digits = 2) {
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

function segmentPointMetric(startX, startY, endX, endY, pointX, pointY) {
  const segmentX = endX - startX;
  const segmentY = endY - startY;
  const lengthSquared = segmentX * segmentX + segmentY * segmentY;
  if (lengthSquared === 0) {
    const dx = pointX - startX;
    const dy = pointY - startY;
    return { distanceSquared: dx * dx + dy * dy, projection: 0 };
  }
  const projection = clamp(
    ((pointX - startX) * segmentX + (pointY - startY) * segmentY) / lengthSquared,
    0,
    1,
  );
  const closestX = startX + segmentX * projection;
  const closestY = startY + segmentY * projection;
  const dx = pointX - closestX;
  const dy = pointY - closestY;
  return { distanceSquared: dx * dx + dy * dy, projection };
}

function publicError(code, message, extra = {}) {
  return { code, message, ...extra };
}

export function normaliseRoomId(value) {
  if (typeof value !== 'string') return null;
  const roomId = value.trim();
  if (roomId.length < 1 || roomId.length > 32) return null;
  return /^[\p{L}\p{N}_-]+$/u.test(roomId) ? roomId : null;
}

export function normalisePlayerName(value) {
  if (typeof value !== 'string') return '游客';
  const name = value.replace(/[\u0000-\u001f\u007f]/g, '').trim().slice(0, 20);
  return name || '游客';
}

function chooseFishType(random) {
  let ticket = random() * FISH_WEIGHT_TOTAL;
  for (const [type, definition] of Object.entries(FISH_TYPES)) {
    ticket -= definition.weight;
    if (ticket <= 0) return [type, definition];
  }
  return ['tuna', FISH_TYPES.tuna];
}

export class GameRoom {
  constructor(roomId, options = {}) {
    this.id = roomId;
    this.random = options.random ?? Math.random;
    this.captureRng = options.captureRng ?? cryptoCaptureRng;
    this.rtp = normaliseRtp(options.rtp ?? DEFAULT_RTP);
    this.now = options.now ?? Date.now;
    this.idFactory = options.idFactory ?? randomUUID;
    this.players = new Map();
    this.fishes = new Map();
    this.bullets = new Map();
    this.events = [];
    this.sequence = 0;
    this.spawnAccumulator = 0;
    this.emptySince = this.now();
    this.lastActiveAt = this.now();

    for (let index = 0; index < 3; index += 1) this.spawnSchool(3, true);
    for (let index = 0; index < 3; index += 1) this.spawnFish(true);
  }

  get playerCount() {
    return this.players.size;
  }

  get targetFishCount() {
    return 12 + this.players.size * 3;
  }

  getPlayer(playerId) {
    return this.players.get(playerId) ?? null;
  }

  addPlayer(data = {}, options = {}) {
    if (this.players.size >= MAX_PLAYERS) {
      return { ok: false, error: publicError('ROOM_FULL', '房间人数已满') };
    }

    const occupiedSeats = new Set([...this.players.values()].map((player) => player.seat));
    const reservedSeats = options.reservedSeats instanceof Set ? options.reservedSeats : new Set();
    const seatUnavailable = (seatNumber) => occupiedSeats.has(seatNumber) || reservedSeats.has(seatNumber);
    let seat = Number.isInteger(data.seat) && !seatUnavailable(data.seat) ? data.seat : -1;
    if (seat < 0 || seat >= MAX_PLAYERS) {
      seat = SEATS.findIndex((_, candidate) => !seatUnavailable(candidate));
    }
    if (seat < 0) return { ok: false, error: publicError('ROOM_FULL', '房间人数已满') };

    const id = typeof data.id === 'string' && data.id ? data.id : this.idFactory();
    const resumeToken = typeof data.resumeToken === 'string' && data.resumeToken
      ? data.resumeToken
      : this.idFactory();
    const bet = normaliseBet(data.bet ?? data.power, 2);
    const player = {
      id,
      uid: Number.isSafeInteger(Number(data.uid)) ? Number(data.uid) : null,
      resumeToken,
      name: normalisePlayerName(data.name),
      seat,
      score: Math.max(0, Math.floor(finiteNumber(data.score) ?? STARTING_SCORE)),
      bet,
      power: bet,
      angle: clampAimToSeat(seat, data.angle ?? SEATS[seat].facing),
      color: PLAYER_COLORS[seat],
      lastFireAt: finiteNumber(data.lastFireAt) ?? Number.NEGATIVE_INFINITY,
      commandResults: data.commandResults instanceof Map ? new Map(data.commandResults) : new Map(),
    };

    this.players.set(player.id, player);
    this.emptySince = null;
    this.lastActiveAt = this.now();
    return { ok: true, player };
  }

  removePlayer(playerId) {
    const player = this.players.get(playerId);
    if (!player) return null;
    this.players.delete(playerId);
    for (const [bulletId, bullet] of this.bullets) {
      if (bullet.ownerId === playerId) this.bullets.delete(bulletId);
    }
    this.lastActiveAt = this.now();
    if (this.players.size === 0) this.emptySince = this.lastActiveAt;
    return player;
  }

  setAim(playerId, angle) {
    const player = this.players.get(playerId);
    const nextAngle = player ? clampAimToSeat(player.seat, angle) : null;
    if (!player || nextAngle === null) return false;
    player.angle = nextAngle;
    this.lastActiveAt = this.now();
    return true;
  }

  setPower(playerId, power) {
    const player = this.players.get(playerId);
    const nextPower = finiteNumber(power);
    if (!player || nextPower === null) return false;
    player.bet = normaliseBet(nextPower, player.bet);
    player.power = player.bet;
    this.lastActiveAt = this.now();
    return true;
  }

  setBet(playerId, bet) {
    return this.setPower(playerId, bet);
  }

  fire(playerId, command = {}, nowMs = this.now(), options = {}) {
    const player = this.players.get(playerId);
    if (!player) {
      return { ok: false, commandId: command?.commandId ?? null, error: publicError('NOT_JOINED', '请先加入房间') };
    }

    const commandId = typeof command?.commandId === 'string' ? command.commandId.trim() : '';
    if (!commandId || commandId.length > 64) {
      return { ok: false, commandId: commandId || null, error: publicError('INVALID_COMMAND_ID', 'commandId 必须是 1 到 64 个字符') };
    }

    // Cache lookup precedes rate limiting so retries receive the exact original ack.
    const previous = player.commandResults.get(commandId);
    if (previous) return previous;

    let result;
    const requestedAngle = command.angle === undefined
      ? player.angle
      : clampAimToSeat(player.seat, command.angle);
    if (requestedAngle === null) {
      result = { ok: false, commandId, error: publicError('INVALID_ANGLE', 'angle 必须是有限数字') };
      this.rememberCommand(player, commandId, result);
      return result;
    }

    const elapsed = nowMs - player.lastFireAt;
    if (elapsed < FIRE_INTERVAL_MS) {
      result = {
        ok: false,
        commandId,
        error: publicError('RATE_LIMITED', '开炮过快', { retryAfterMs: Math.ceil(FIRE_INTERVAL_MS - elapsed) }),
      };
      this.rememberCommand(player, commandId, result);
      return result;
    }

    if (player.score < player.bet) {
      result = {
        ok: false,
        commandId,
        error: publicError('INSUFFICIENT_SCORE', '积分不足，无法开炮'),
      };
      this.rememberCommand(player, commandId, result);
      return result;
    }

    player.angle = requestedAngle;
    player.lastFireAt = nowMs;
    player.score -= player.bet;
    const origin = SEATS[player.seat];
    const muzzleDistance = 34;
    const bullet = {
      id: this.idFactory(),
      ownerId: player.id,
      x: origin.x + Math.cos(player.angle) * muzzleDistance,
      y: origin.y + Math.sin(player.angle) * muzzleDistance,
      previousX: origin.x + Math.cos(player.angle) * muzzleDistance,
      previousY: origin.y + Math.sin(player.angle) * muzzleDistance,
      vx: Math.cos(player.angle) * BULLET_SPEED,
      vy: Math.sin(player.angle) * BULLET_SPEED,
      angle: player.angle,
      bet: player.bet,
      power: player.bet,
      radius: 7 + Math.log2(player.bet + 1) * 1.8,
      age: 0,
      pendingWallet: Boolean(options.deferWallet),
    };
    this.bullets.set(bullet.id, bullet);
    this.lastActiveAt = nowMs;

    result = {
      ok: true,
      commandId,
      bulletId: bullet.id,
      shotId: bullet.id,
      bet: bullet.bet,
      power: bullet.bet,
      score: player.score,
      serverTime: nowMs,
    };
    this.rememberCommand(player, commandId, result);
    return result;
  }

  commitFire(playerId, commandId, walletBalance) {
    const player = this.players.get(playerId);
    const result = player?.commandResults.get(commandId);
    const bullet = result?.bulletId ? this.bullets.get(result.bulletId) : null;
    if (!player || !result?.ok || !bullet) return false;
    bullet.pendingWallet = false;
    if (Number.isFinite(Number(walletBalance))) {
      player.score = Math.max(0, Math.floor(Number(walletBalance)));
      result.score = player.score;
    }
    return true;
  }

  rollbackFire(playerId, commandId) {
    const player = this.players.get(playerId);
    const result = player?.commandResults.get(commandId);
    if (!player || !result?.ok) return false;
    this.bullets.delete(result.bulletId);
    player.score += Number(result.bet || 0);
    player.commandResults.delete(commandId);
    return true;
  }

  rememberCommand(player, commandId, result) {
    player.commandResults.set(commandId, result);
    while (player.commandResults.size > COMMAND_CACHE_SIZE) {
      player.commandResults.delete(player.commandResults.keys().next().value);
    }
  }

  spawnFish(initial = false, options = {}) {
    const selectedType = FISH_TYPES[options.type] ? options.type : null;
    const [type, definition] = selectedType
      ? [selectedType, FISH_TYPES[selectedType]]
      : chooseFishType(this.random);
    const fromLeft = typeof options.fromLeft === 'boolean' ? options.fromLeft : this.random() >= 0.5;
    const margin = definition.radius * 2;
    const y = clamp(finiteNumber(options.y) ?? 82 + this.random() * 475, 72, 575);
    const speedVariance = finiteNumber(options.speedVariance) ?? 0.86 + this.random() * 0.3;
    const direction = fromLeft ? 1 : -1;
    const fish = {
      id: this.idFactory(),
      type,
      x: finiteNumber(options.x)
        ?? (initial ? this.random() * WORLD_WIDTH : (fromLeft ? -margin : WORLD_WIDTH + margin)),
      y,
      baseY: y,
      vx: direction * definition.speed * speedVariance,
      angle: fromLeft ? 0 : Math.PI,
      scale: definition.scale * (0.92 + this.random() * 0.16),
      multiplier: definition.multiplier,
      tier: definition.tier,
      assetKey: definition.assetKey,
      radius: definition.radius,
      age: this.random() * 10,
      waveAmplitude: finiteNumber(options.waveAmplitude) ?? 7 + this.random() * 20,
      waveSpeed: finiteNumber(options.waveSpeed) ?? 0.6 + this.random() * 1.15,
      phase: finiteNumber(options.phase) ?? this.random() * Math.PI * 2,
      schoolId: typeof options.schoolId === 'string' ? options.schoolId : null,
    };
    this.fishes.set(fish.id, fish);
    return fish;
  }

  spawnSchool(size = 3, initial = false) {
    const count = clamp(Math.floor(finiteNumber(size) ?? 3), 2, SCHOOL_OFFSETS.length);
    const typeIndex = Math.floor(this.random() * SCHOOL_FISH_TYPES.length);
    const type = SCHOOL_FISH_TYPES[clamp(typeIndex, 0, SCHOOL_FISH_TYPES.length - 1)];
    const definition = FISH_TYPES[type];
    const fromLeft = this.random() >= 0.5;
    const direction = fromLeft ? 1 : -1;
    const anchorY = 130 + this.random() * 360;
    const speedVariance = 0.9 + this.random() * 0.18;
    const waveAmplitude = 6 + this.random() * 10;
    const waveSpeed = 0.65 + this.random() * 0.5;
    const phase = this.random() * Math.PI * 2;
    const schoolId = `school-${this.idFactory()}`;
    const edgeX = fromLeft ? -definition.radius * 2 : WORLD_WIDTH + definition.radius * 2;
    const anchorX = initial ? this.random() * WORLD_WIDTH : edgeX;

    return SCHOOL_OFFSETS.slice(0, count).map((offset) => this.spawnFish(initial, {
      type,
      fromLeft,
      x: anchorX - direction * offset.behind,
      y: anchorY + offset.y,
      speedVariance,
      waveAmplitude,
      waveSpeed,
      phase,
      schoolId,
    }));
  }

  tick(deltaSeconds) {
    const dt = clamp(finiteNumber(deltaSeconds) ?? 0, 0, 0.1);
    if (dt <= 0) return;

    this.spawnAccumulator += dt;
    for (const [fishId, fish] of this.fishes) {
      fish.age += dt;
      fish.x += fish.vx * dt;
      fish.y = fish.baseY + Math.sin(fish.age * fish.waveSpeed + fish.phase) * fish.waveAmplitude;
      if (fish.x < -170 || fish.x > WORLD_WIDTH + 170) this.fishes.delete(fishId);
    }

    for (const [bulletId, bullet] of this.bullets) {
      if (bullet.pendingWallet) continue;
      bullet.age += dt;
      bullet.previousX = bullet.x;
      bullet.previousY = bullet.y;
      bullet.x += bullet.vx * dt;
      bullet.y += bullet.vy * dt;
    }

    this.resolveCollisions();

    // Cull after collision resolution so a final segment crossing the world edge can still hit.
    for (const [bulletId, bullet] of this.bullets) {
      if (bullet.pendingWallet) continue;
      if (
        bullet.age > BULLET_LIFETIME_SECONDS
        || bullet.x < -80
        || bullet.x > WORLD_WIDTH + 80
        || bullet.y < -80
        || bullet.y > WORLD_HEIGHT + 80
      ) {
        this.bullets.delete(bulletId);
      }
    }

    const deficit = this.targetFishCount - this.fishes.size;
    if (deficit > 0 && this.spawnAccumulator >= 0.22) {
      if (deficit >= 3 && this.random() < 0.42) {
        this.spawnSchool(Math.min(deficit, 3), false);
      } else {
        const spawnCount = Math.min(deficit, Math.max(1, Math.floor(this.spawnAccumulator / 0.22)));
        for (let index = 0; index < spawnCount; index += 1) this.spawnFish(false);
      }
      this.spawnAccumulator = 0;
    }
  }

  resolveCollisions() {
    for (const [bulletId, bullet] of this.bullets) {
      let hitFish = null;
      let nearestProjection = Number.POSITIVE_INFINITY;
      let nearestDistanceSquared = Number.POSITIVE_INFINITY;
      for (const fish of this.fishes.values()) {
        const metric = segmentPointMetric(
          bullet.previousX ?? bullet.x,
          bullet.previousY ?? bullet.y,
          bullet.x,
          bullet.y,
          fish.x,
          fish.y,
        );
        const collisionRadius = bullet.radius + fish.radius * fish.scale;
        if (
          metric.distanceSquared <= collisionRadius * collisionRadius
          && (
            metric.projection < nearestProjection
            || (metric.projection === nearestProjection && metric.distanceSquared < nearestDistanceSquared)
          )
        ) {
          nearestProjection = metric.projection;
          nearestDistanceSquared = metric.distanceSquared;
          hitFish = fish;
        }
      }
      if (!hitFish) continue;

      this.bullets.delete(bulletId);
      const player = this.players.get(bullet.ownerId);
      if (!player) continue;
      const multiplier = Math.max(1, finiteNumber(hitFish.multiplier) ?? 1);
      const { captured } = resolveCapture({
        multiplier,
        rtp: this.rtp,
        captureRng: this.captureRng,
      });
      const reward = captured ? bullet.bet * multiplier : 0;
      if (captured) {
        // Room simulation is single-threaded: removing the fish here makes this
        // capture atomic before another bullet can inspect the remaining map.
        this.fishes.delete(hitFish.id);
        player.score += reward;
      }
      this.events.push({
        resolutionId: this.idFactory(),
        roomId: this.id,
        playerId: player.id,
        playerName: player.name,
        shotId: bullet.id,
        bulletId: bullet.id,
        fishId: hitFish.id,
        type: hitFish.type,
        fishType: hitFish.type,
        assetKey: hitFish.assetKey,
        tier: hitFish.tier,
        captured,
        multiplier,
        bet: bullet.bet,
        power: bullet.bet,
        reward,
        payout: reward,
        score: player.score,
        x: round(hitFish.x),
        y: round(hitFish.y),
        serverTime: this.now(),
      });
    }
  }

  drainEvents() {
    return this.events.splice(0, this.events.length);
  }

  snapshot(advanceSequence = true) {
    if (advanceSequence) this.sequence += 1;
    return {
      seq: this.sequence,
      serverTime: this.now(),
      roomId: this.id,
      width: WORLD_WIDTH,
      height: WORLD_HEIGHT,
      seatLayout: 'four-seat-top-bottom',
      rtp: this.rtp,
      betLevels: BET_LEVELS,
      seats: SEATS.map((seat) => ({
        seat: seat.seat,
        side: seat.side,
        position: seat.position,
        x: seat.x,
        y: seat.y,
        facing: round(seat.facing, 4),
        minAimOffset: round(seat.minAimOffset, 4),
        maxAimOffset: round(seat.maxAimOffset, 4),
      })),
      players: [...this.players.values()]
        .sort((left, right) => left.seat - right.seat)
        .map((player) => ({
          id: player.id,
          name: player.name,
          seat: player.seat,
          score: player.score,
          bet: player.bet,
          power: player.power,
          angle: round(player.angle, 4),
          color: player.color,
          side: SEATS[player.seat].side,
          position: SEATS[player.seat].position,
          x: SEATS[player.seat].x,
          y: SEATS[player.seat].y,
          facing: round(SEATS[player.seat].facing, 4),
        })),
      fishes: [...this.fishes.values()].map((fish) => ({
        id: fish.id,
        type: fish.type,
        x: round(fish.x),
        y: round(fish.y),
        angle: round(fish.angle, 4),
        scale: round(fish.scale, 3),
        radius: fish.radius,
        multiplier: fish.multiplier,
        // Legacy clients used reward as a fixed fish label. Keep it as a
        // multiplier alias; the authoritative payout remains bet * multiplier.
        reward: fish.multiplier,
        tier: fish.tier,
        assetKey: fish.assetKey,
        schoolId: fish.schoolId,
      })),
      bullets: [...this.bullets.values()].filter((bullet) => !bullet.pendingWallet).map((bullet) => ({
        id: bullet.id,
        ownerId: bullet.ownerId,
        x: round(bullet.x),
        y: round(bullet.y),
        angle: round(bullet.angle, 4),
        bet: bullet.bet,
        power: bullet.power,
      })),
    };
  }
}

export class GameEngine {
  constructor(options = {}) {
    this.rooms = new Map();
    this.resumeSessions = new Map();
    this.random = options.random ?? Math.random;
    this.captureRng = options.captureRng ?? cryptoCaptureRng;
    this.rtp = normaliseRtp(options.rtp ?? DEFAULT_RTP);
    this.now = options.now ?? Date.now;
    this.idFactory = options.idFactory ?? randomUUID;
    this.resumeTtlMs = options.resumeTtlMs ?? RESUME_TTL_MS;
    this.emptyRoomTtlMs = options.emptyRoomTtlMs ?? EMPTY_ROOM_TTL_MS;
    this.maxRooms = Math.max(1, Math.floor(finiteNumber(options.maxRooms) ?? DEFAULT_MAX_ROOMS));
  }

  createRoom(roomId) {
    const room = new GameRoom(roomId, {
      random: this.random,
      captureRng: this.captureRng,
      rtp: this.rtp,
      now: this.now,
      idFactory: this.idFactory,
    });
    this.rooms.set(roomId, room);
    return room;
  }

  getRoom(roomId) {
    return this.rooms.get(roomId) ?? null;
  }

  join(input = {}) {
    const roomId = normaliseRoomId(input.roomId);
    if (!roomId) {
      return this.joinFailure(input.roomId ?? null, 'INVALID_ROOM', 'roomId 只能包含文字、数字、下划线或短横线，最长 32 个字符');
    }

    const nowMs = this.now();
    this.prune(nowMs);
    let room = this.rooms.get(roomId);
    let resumedData = null;
    const suppliedToken = typeof input.resumeToken === 'string' ? input.resumeToken : '';
    if (suppliedToken) {
      const session = this.resumeSessions.get(suppliedToken);
      if (session && session.roomId === roomId && session.expiresAt > nowMs) {
        resumedData = session.player;
      }
    }

    if (!room && this.rooms.size >= this.maxRooms) {
      return this.joinFailure(roomId, 'SERVER_FULL', '活跃房间已达到上限，请稍后重试');
    }
    if (!room) room = this.createRoom(roomId);
    const reservedSeats = this.getReservedSeats(roomId, resumedData ? suppliedToken : null, nowMs);
    const addition = room.addPlayer(resumedData
      ? { ...resumedData, name: input.name || resumedData.name }
      : {
        name: input.name,
        uid: input.uid,
        score: input.score,
        seat: input.seat ?? input.requestedSeat
      }, { reservedSeats });
    if (!addition.ok) {
      return this.joinFailure(roomId, addition.error.code, addition.error.message, room.snapshot(false));
    }

    const player = addition.player;
    if (resumedData) this.resumeSessions.delete(suppliedToken);
    return {
      ok: true,
      roomId,
      playerId: player.id,
      seat: player.seat,
      side: SEATS[player.seat].side,
      position: SEATS[player.seat].position,
      resumeToken: player.resumeToken,
      state: room.snapshot(false),
      resumed: Boolean(resumedData),
    };
  }

  joinFailure(roomId, code, message, state = null) {
    return {
      ok: false,
      roomId,
      playerId: null,
      resumeToken: null,
      state,
      error: publicError(code, message),
    };
  }

  leave(roomId, playerId) {
    const room = this.rooms.get(roomId);
    if (!room) return null;
    const player = room.removePlayer(playerId);
    if (!player) return null;

    const resumablePlayer = {
      id: player.id,
      uid: player.uid,
      resumeToken: player.resumeToken,
      name: player.name,
      seat: player.seat,
      score: player.score,
      power: player.power,
      bet: player.bet,
      angle: player.angle,
      lastFireAt: player.lastFireAt,
      commandResults: new Map(player.commandResults),
    };
    this.resumeSessions.set(player.resumeToken, {
      roomId,
      player: resumablePlayer,
      expiresAt: this.now() + this.resumeTtlMs,
    });
    return resumablePlayer;
  }

  getReservedSeats(roomId, exceptToken = null, nowMs = this.now()) {
    const seats = new Set();
    for (const [token, session] of this.resumeSessions) {
      if (
        token !== exceptToken
        && session.roomId === roomId
        && session.expiresAt > nowMs
        && Number.isInteger(session.player.seat)
      ) {
        seats.add(session.player.seat);
      }
    }
    return seats;
  }

  tick(deltaSeconds) {
    const nowMs = this.now();
    for (const room of this.rooms.values()) room.tick(deltaSeconds);
    this.prune(nowMs);
  }

  prune(nowMs = this.now()) {
    for (const [token, session] of this.resumeSessions) {
      if (session.expiresAt <= nowMs) this.resumeSessions.delete(token);
    }
    for (const [roomId, room] of this.rooms) {
      if (room.playerCount === 0 && room.emptySince !== null && nowMs - room.emptySince >= this.emptyRoomTtlMs) {
        this.rooms.delete(roomId);
      }
    }
  }

  stats() {
    let players = 0;
    let fishes = 0;
    let bullets = 0;
    for (const room of this.rooms.values()) {
      players += room.players.size;
      fishes += room.fishes.size;
      bullets += room.bullets.size;
    }
    return {
      rooms: this.rooms.size,
      maxRooms: this.maxRooms,
      players,
      fishes,
      bullets,
      targetRtp: this.rtp,
    };
  }
}

export function createGameEngine(options = {}) {
  return new GameEngine(options);
}
