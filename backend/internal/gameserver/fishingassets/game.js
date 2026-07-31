import {
  displaySeatToServerSeat,
  normalizeSeat,
  serverSeatToDisplaySeat,
  viewToWorldPoint,
  worldToViewAngle,
  worldToViewPoint
} from "./perspective.js?v=20260731-local-view1";

const WORLD = Object.freeze({ width: 1280, height: 720 });
const ROOM_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
const POWER_LEVELS = Object.freeze([1, 2, 5, 10, 20, 50]);
const DEFAULT_COLORS = Object.freeze(["#55e7f0", "#f5c65f", "#ff7188", "#a88cff"]);
const TARGET_RENDER_FPS = 30;
const RENDER_FRAME_INTERVAL = 1000 / TARGET_RENDER_FPS;
const PORTRAIT_LANDSCAPE_QUERY = window.matchMedia("(max-width: 700px) and (orientation: portrait)");
const REDUCED_MOTION_QUERY = window.matchMedia("(prefers-reduced-motion: reduce)");

// Fallback physics origins mirror the two-up/two-down four-seat cabinet.
// Authoritative player.x/y coordinates take precedence when present.
const LEGACY_PHYSICS_SEATS = Object.freeze([
  Object.freeze({ x: 430, y: 690 }),
  Object.freeze({ x: 850, y: 690 }),
  Object.freeze({ x: 430, y: 30 }),
  Object.freeze({ x: 850, y: 30 })
]);

const DISPLAY_SEATS = Object.freeze([
  Object.freeze({ x: 430, y: 690, side: "bottom-left", inward: -Math.PI / 2 }),
  Object.freeze({ x: 850, y: 690, side: "bottom-right", inward: -Math.PI / 2 }),
  Object.freeze({ x: 430, y: 30, side: "top-left", inward: Math.PI / 2 }),
  Object.freeze({ x: 850, y: 30, side: "top-right", inward: Math.PI / 2 })
]);

const SPECIES = Object.freeze({
  tuna: Object.freeze({ asset: "fish1", frameWidth: 55, frameHeight: 37, swimFrames: 4, fps: 5, name: "小黄鱼", multiplier: 2, size: 1.15, drift: 2.1 }),
  lionfish: Object.freeze({ asset: "fish2", frameWidth: 78, frameHeight: 64, swimFrames: 4, fps: 5, name: "热带鱼", multiplier: 3, size: 1.2, drift: 2.5 }),
  puffer: Object.freeze({ asset: "fish3", frameWidth: 72, frameHeight: 56, swimFrames: 4, fps: 5, name: "河豚", multiplier: 5, size: 1.16, drift: 2.7 }),
  grouper: Object.freeze({ asset: "fish4", frameWidth: 77, frameHeight: 59, swimFrames: 4, fps: 5, name: "深海石斑", multiplier: 8, size: 1.24, drift: 2.1 }),
  turtle: Object.freeze({ asset: "fish10", frameWidth: 178, frameHeight: 187, swimFrames: 6, fps: 4, name: "玳瑁海龟", multiplier: 12, size: 1.3, drift: 2.4 }),
  manta: Object.freeze({ asset: "fish9", frameWidth: 166, frameHeight: 183, swimFrames: 8, fps: 4, name: "黄金蝠鲼", multiplier: 20, size: 1.38, drift: 3 }),
  hammerhead: Object.freeze({ asset: "shark1", frameWidth: 509, frameHeight: 270, swimFrames: 8, fps: 4, name: "锤头鲨", multiplier: 30, size: 1.52, boss: true, drift: 1.9 }),
  octopus: Object.freeze({ asset: "fish7", frameWidth: 92, frameHeight: 151, swimFrames: 6, fps: 4, name: "梦幻水母", multiplier: 40, size: 1.56, boss: true, drift: 2.8 }),
  orca: Object.freeze({ asset: "shark2", frameWidth: 516, frameHeight: 273, swimFrames: 8, fps: 4, name: "黄金锤头鲨", multiplier: 60, size: 1.66, boss: true, drift: 1.7 }),
  anglerfish: Object.freeze({ asset: "fish8", frameWidth: 174, frameHeight: 126, swimFrames: 8, fps: 4, name: "灯笼鱼", multiplier: 80, size: 1.58, boss: true, drift: 2.2 })
});

const CANNON_PALETTES = Object.freeze([
  Object.freeze({ light: "#aaf8ff", body: "#25c9e6", dark: "#075487", trim: "#ffe47c" }),
  Object.freeze({ light: "#e0ff9d", body: "#69d948", dark: "#23782c", trim: "#fff093" }),
  Object.freeze({ light: "#fff1a1", body: "#ffbd31", dark: "#b75a12", trim: "#fff4b0" }),
  Object.freeze({ light: "#ffd0a1", body: "#ff792f", dark: "#a9311c", trim: "#ffe681" }),
  Object.freeze({ light: "#ffb7e7", body: "#ed4da8", dark: "#84266f", trim: "#fff0a0" }),
  Object.freeze({ light: "#dec9ff", body: "#8c62ec", dark: "#452295", trim: "#ffe970" })
]);

const LEGACY_TYPE_MAP = Object.freeze({
  blue: "tuna",
  green: "lionfish",
  orange: "puffer",
  pink: "grouper",
  red: "turtle",
  brown: "manta",
  grey: "hammerhead",
  eel: "octopus"
});

const canvas = document.querySelector("#gameCanvas");
const ctx = canvas.getContext("2d", { alpha: false });
ctx.imageSmoothingEnabled = true;
ctx.imageSmoothingQuality = "high";

const ui = {
  gameFrame: document.querySelector("#gameFrame"),
  connectionBadge: document.querySelector("#connectionBadge"),
  roomButton: document.querySelector("#roomButton"),
  roomLabel: document.querySelector("#roomLabel"),
  playerCount: document.querySelector("#playerCount"),
  eventFeed: document.querySelector("#eventFeed"),
  powerValue: document.querySelector("#powerValue"),
  powerDown: document.querySelector("#powerDown"),
  powerUp: document.querySelector("#powerUp"),
  lockButton: document.querySelector("#lockButton"),
  autoButton: document.querySelector("#autoButton"),
  fireButton: document.querySelector("#fireButton"),
  crosshair: document.querySelector("#crosshair"),
  reconnectMask: document.querySelector("#reconnectMask"),
  joinOverlay: document.querySelector("#joinOverlay"),
  joinForm: document.querySelector("#joinForm"),
  nameInput: document.querySelector("#nameInput"),
  roomInput: document.querySelector("#roomInput"),
  randomRoom: document.querySelector("#randomRoom"),
  joinButton: document.querySelector("#joinButton"),
  joinError: document.querySelector("#joinError"),
  toast: document.querySelector("#toast"),
  seats: [...document.querySelectorAll(".seat-hud")]
};

const images = {
  arena: loadImage("assets/arcade-ocean-arena-v3.png"),
  fish1: loadImage("assets/fish-animated/fish1.png"),
  fish2: loadImage("assets/fish-animated/fish2.png"),
  fish3: loadImage("assets/fish-animated/fish3.png"),
  fish4: loadImage("assets/fish-animated/fish4.png"),
  fish7: loadImage("assets/fish-animated/fish7.png"),
  fish8: loadImage("assets/fish-animated/fish8.png"),
  fish9: loadImage("assets/fish-animated/fish9.png"),
  fish10: loadImage("assets/fish-animated/fish10.png"),
  shark1: loadImage("assets/fish-animated/shark1.png"),
  shark2: loadImage("assets/fish-animated/shark2.png")
};

const state = {
  joined: false,
  joining: false,
  roomId: "",
  playerId: "",
  localSeat: null,
  resumeToken: "",
  profile: null,
  current: null,
  previous: null,
  snapshotAt: 0,
  pointer: { x: WORLD.width / 2, y: WORLD.height * 0.45, active: false },
  lastAimSentAt: 0,
  lastLocalFireAt: 0,
  selectedPower: 2,
  powerTouched: false,
  lockSeeking: false,
  lockedFishId: "",
  autoFire: false,
  autoTimer: 0,
  effects: [],
  muzzleFlashes: [],
  fishFlashes: new Map(),
  fishMotion: new Map(),
  lastFishMotionSweepAt: 0,
  bulletHistory: new Map(),
  visualAimAngles: new Map(),
  toastTimer: 0
};

const particles = Array.from({ length: 76 }, (_, index) => ({
  x: (index * 251 + 43) % WORLD.width,
  y: (index * 149 + 71) % WORLD.height,
  radius: 0.7 + ((index * 13) % 17) / 9,
  speed: 3 + ((index * 19) % 13),
  drift: 5 + ((index * 29) % 20),
  phase: index * 0.73,
  alpha: 0.08 + ((index * 7) % 10) / 90
}));

const bubbles = Array.from({ length: 25 }, (_, index) => ({
  x: (index * 317 + 88) % WORLD.width,
  y: (index * 199 + 37) % WORLD.height,
  radius: 1.6 + ((index * 11) % 16) / 4,
  speed: 9 + ((index * 23) % 26),
  phase: index * 0.91
}));

// 依据当前页面所在目录推导 socket.io 路径，兼容根挂载与 /minigame/fish/ 子路径
const socketPath = `${location.pathname.replace(/\/[^/]*$/, '')}/socket.io`.replace(/\/{2,}/g, '/');

const socket = window.io({
  path: socketPath,
  autoConnect: false,
  reconnection: true,
  reconnectionDelay: 500,
  reconnectionDelayMax: 3000,
  reconnectionAttempts: Infinity,
  timeout: 8000
});

function loadImage(src) {
  const image = new Image();
  image.decoding = "async";
  image.src = src;
  return image;
}

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(maximum, value));
}

function lerp(from, to, amount) {
  return from + (to - from) * amount;
}

function stableUnit(value) {
  const textValue = String(value || "fish");
  let hash = 2166136261;
  for (let index = 0; index < textValue.length; index += 1) {
    hash ^= textValue.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0) / 4294967296;
}

function shortestAngleDelta(target, current) {
  return Math.atan2(Math.sin(target - current), Math.cos(target - current));
}

function asArray(value) {
  if (Array.isArray(value)) return value;
  if (value && typeof value === "object") return Object.values(value);
  return [];
}

function randomRoomId() {
  const bytes = crypto.getRandomValues(new Uint8Array(5));
  return Array.from(bytes, (value) => ROOM_ALPHABET[value % ROOM_ALPHABET.length]).join("");
}

function normalizeRoomId(value) {
  return String(value || "")
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, "")
    .slice(0, 6);
}

function normalizeName(value) {
  return String(value || "").replace(/[<>]/g, "").replace(/\s+/g, " ").trim().slice(0, 12);
}

function setConnectionStatus(kind, label) {
  ui.connectionBadge.className = `status-pill ${kind}`;
  ui.connectionBadge.querySelector("span").textContent = label;
}

function showJoinError(message) {
  ui.joinError.textContent = message;
  ui.joinError.hidden = !message;
}

function setJoining(joining) {
  state.joining = joining;
  ui.joinButton.disabled = joining;
  ui.joinButton.querySelector("span").textContent = joining ? "正在连接战场…" : "进入深海猎场";
}

function showToast(message) {
  clearTimeout(state.toastTimer);
  ui.toast.textContent = message;
  ui.toast.classList.add("show");
  state.toastTimer = window.setTimeout(() => ui.toast.classList.remove("show"), 1800);
}

function addFeed(message, accent = "") {
  const item = document.createElement("div");
  item.className = "feed-item";
  if (accent && message.includes(accent)) {
    const [before, ...after] = message.split(accent);
    item.append(document.createTextNode(before));
    const strong = document.createElement("strong");
    strong.textContent = accent;
    item.append(strong, document.createTextNode(after.join(accent)));
  } else {
    item.textContent = message;
  }
  ui.eventFeed.prepend(item);
  while (ui.eventFeed.children.length > 4) ui.eventFeed.lastElementChild.remove();
  window.setTimeout(() => item.remove(), 5200);
}

function playersFrom(snapshot = state.current) {
  return asArray(snapshot?.players);
}

function fishesFrom(snapshot = state.current) {
  return asArray(snapshot?.fishes || snapshot?.fish);
}

function bulletsFrom(snapshot = state.current) {
  return asArray(snapshot?.bullets);
}

function currentPlayer(snapshot = state.current) {
  return playersFrom(snapshot).find((player) => String(player.id) === String(state.playerId));
}

function viewerSeat() {
  const snapshotSeat = Number(currentPlayer()?.seat);
  if (Number.isFinite(snapshotSeat)) return normalizeSeat(snapshotSeat);
  const assignedSeat = Number(state.localSeat);
  return state.localSeat !== null && Number.isFinite(assignedSeat) ? normalizeSeat(assignedSeat) : 0;
}

function toViewPoint(point) {
  return worldToViewPoint(point, viewerSeat(), WORLD);
}

function toWorldPoint(point) {
  return viewToWorldPoint(point, viewerSeat(), WORLD);
}

function toViewAngle(angle) {
  return worldToViewAngle(angle, viewerSeat());
}

function playerColor(player) {
  return player?.color || DEFAULT_COLORS[Number(player?.seat || 0) % DEFAULT_COLORS.length];
}

function canonicalSeatOrigin(seat) {
  return DISPLAY_SEATS[normalizeSeat(seat)];
}

function displaySeatOrigin(serverSeat) {
  return DISPLAY_SEATS[serverSeatToDisplaySeat(serverSeat, viewerSeat())];
}

function physicsSeatOrigin(player) {
  if (Number.isFinite(player?.x) && Number.isFinite(player?.y)) return { x: player.x, y: player.y };
  const layout = String(state.current?.seatLayout || state.current?.layout || "").toLowerCase();
  if (layout.includes("four") || layout.includes("table")) return canonicalSeatOrigin(player?.seat);
  return LEGACY_PHYSICS_SEATS[normalizeSeat(player?.seat)];
}

function fishAssetKey(fish = {}) {
  const supplied = String(fish.assetKey || fish.species || fish.kind || "").toLowerCase();
  if (SPECIES[supplied]) return supplied;
  const type = String(fish.type || "").toLowerCase();
  if (SPECIES[type]) return type;
  return LEGACY_TYPE_MAP[type] || "tuna";
}

function fishSpec(fish = {}) {
  const key = fishAssetKey(fish);
  return { key, ...SPECIES[key] };
}

function fishMotionFor(fish, spec, now) {
  const id = String(fish.id || spec.key);
  let motion = state.fishMotion.get(id);
  if (!motion) {
    const seed = stableUnit(`${spec.key}:${id}`);
    const secondSeed = (seed * 37.173 + 0.217) % 1;
    const thirdSeed = (seed * 83.719 + 0.631) % 1;
    motion = {
      phase: seed * Math.PI * 2,
      rateScale: 0.88 + secondSeed * 0.27,
      amplitudeScale: 0.82 + thirdSeed * 0.34,
      pitch: 0,
      updatedAt: now,
      lastSeenAt: now
    };
    state.fishMotion.set(id, motion);
  }
  motion.lastSeenAt = now;
  return motion;
}

function fishMultiplier(fish = {}) {
  const spec = fishSpec(fish);
  const supplied = Number(fish.multiplier ?? fish.odds ?? fish.payoutMultiplier);
  return Math.max(1, Math.round(Number.isFinite(supplied) ? supplied : spec.multiplier));
}

function closestPowerIndex(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return 1;
  let bestIndex = 0;
  for (let index = 1; index < POWER_LEVELS.length; index += 1) {
    if (Math.abs(POWER_LEVELS[index] - numeric) < Math.abs(POWER_LEVELS[bestIndex] - numeric)) bestIndex = index;
  }
  return bestIndex;
}

function setModeButton(button, active) {
  button.classList.toggle("active", active);
  button.setAttribute("aria-pressed", String(active));
}

function updateHud() {
  const players = playersFrom();
  const me = currentPlayer();
  if (me && !state.powerTouched) state.selectedPower = POWER_LEVELS[closestPowerIndex(me.power)];
  ui.playerCount.textContent = String(players.length);
  ui.powerValue.textContent = String(state.selectedPower);

  const bySeat = new Map(players.map((player) => [Number(player.seat), player]));
  for (const hud of ui.seats) {
    const displaySeat = normalizeSeat(hud.dataset.seat);
    const serverSeat = displaySeatToServerSeat(displaySeat, viewerSeat());
    const player = bySeat.get(serverSeat);
    const mine = player && String(player.id) === String(state.playerId);
    hud.dataset.serverSeat = String(serverSeat);
    hud.classList.toggle("occupied", Boolean(player));
    hud.classList.toggle("current", Boolean(mine));
    hud.style.setProperty("--seat-color", playerColor(player || { seat: serverSeat }));
    hud.querySelector(".seat-number").textContent = String(serverSeat + 1).padStart(2, "0");
    hud.querySelector(".seat-name").textContent = player ? String(player.name || "匿名猎手") : "等待入座";
    hud.querySelector(".seat-score").textContent = player
      ? Math.max(0, Math.floor(Number(player.score || 0))).toLocaleString("zh-CN")
      : "—";
    hud.querySelector(".seat-power b").textContent = String(mine ? state.selectedPower : Math.max(1, Number(player?.power || 1)));
  }

  setModeButton(ui.lockButton, state.lockSeeking || Boolean(state.lockedFishId));
  setModeButton(ui.autoButton, state.autoFire);
}

function setSnapshot(snapshot, { force = false } = {}) {
  if (!snapshot || typeof snapshot !== "object") return;
  const sameRoom = state.current && String(snapshot.roomId || "") === String(state.current.roomId || "");
  if (!force && sameRoom && Number(snapshot.seq || 0) < Number(state.current.seq || 0)) return;

  if (force || !sameRoom) {
    state.previous = null;
    state.current = null;
    state.bulletHistory.clear();
    state.effects.length = 0;
    state.muzzleFlashes.length = 0;
    state.fishFlashes.clear();
    state.fishMotion.clear();
    state.lastFishMotionSweepAt = 0;
    state.visualAimAngles.clear();
    state.lockedFishId = "";
    state.lockSeeking = false;
  } else {
    state.previous = state.current;
  }

  state.current = snapshot;
  const snapshotPlayer = currentPlayer(snapshot);
  if (Number.isFinite(Number(snapshotPlayer?.seat))) state.localSeat = normalizeSeat(snapshotPlayer.seat);
  state.snapshotAt = performance.now();
  if (state.lockedFishId && !fishesFrom(snapshot).some((fish) => String(fish.id) === state.lockedFishId)) {
    state.lockedFishId = "";
  }
  updateHud();
}

function submitJoin() {
  const name = normalizeName(ui.nameInput.value);
  const roomId = normalizeRoomId(ui.roomInput.value);
  if (!name) return showJoinError("请先输入昵称。"), false;
  if (roomId.length < 4) return showJoinError("房间号至少需要 4 位。"), false;

  showJoinError("");
  setJoining(true);
  state.profile = { name, roomId };
  state.roomId = roomId;
  localStorage.setItem("ocean-hunters:name", name);
  history.replaceState(null, "", `${location.pathname}?room=${encodeURIComponent(roomId)}`);
  try {
    const savedSession = JSON.parse(localStorage.getItem(`ocean-hunters:resume:${roomId}`) || "null");
    state.resumeToken = savedSession?.name === name ? String(savedSession.token || "") : "";
  } catch {
    state.resumeToken = "";
  }
  setConnectionStatus("connecting", "连接中");
  if (socket.connected) emitJoin();
  else socket.connect();
  return true;
}

function resetToJoin(errorMessage) {
  state.joined = false;
  state.playerId = "";
  state.localSeat = null;
  state.current = null;
  state.previous = null;
  state.bulletHistory.clear();
  state.fishMotion.clear();
  state.lastFishMotionSweepAt = 0;
  state.lockedFishId = "";
  state.autoFire = false;
  syncAutoTimer();
  ui.reconnectMask.hidden = true;
  ui.joinOverlay.classList.remove("hidden");
  showJoinError(errorMessage || "");
}

function emitJoin() {
  if (!state.profile || !socket.connected) return;
  const payload = {
    roomId: state.profile.roomId,
    name: state.profile.name,
    resumeToken: state.resumeToken || undefined,
    uid: state.profile.uid,
    ts: state.profile.ts,
    sig: state.profile.sig,
    table: state.profile.table,
    seat: state.profile.seat,
    session: state.profile.session,
    resume: state.profile.resume,
    venue: state.profile.venue
  };

  socket.timeout(8000).emit("room:join", payload, (timeoutError, response) => {
    if (timeoutError) {
      setJoining(false);
      const message = "房间服务响应超时，请检查服务是否已启动。";
      if (state.joined) resetToJoin(message);
      else showJoinError(message);
      setConnectionStatus("offline", "需重新加入");
      return;
    }

    if (!response?.ok) {
      const message = response?.error?.message || response?.error || "无法进入房间，请换一个房间号重试。";
      setJoining(false);
      if (state.joined) resetToJoin(message);
      else showJoinError(message);
      if (response?.error?.code === "ROOM_FULL" || response?.error?.code === "INVALID_ROOM") {
        state.resumeToken = "";
        localStorage.removeItem(`ocean-hunters:resume:${state.roomId}`);
      }
      setConnectionStatus("offline", "需重新加入");
      return;
    }

    state.joined = true;
    state.roomId = String(response.roomId || state.profile.roomId);
    state.playerId = String(response.playerId || "");
    if (response.seat !== null && response.seat !== undefined && Number.isFinite(Number(response.seat))) {
      state.localSeat = normalizeSeat(response.seat);
    }
    state.resumeToken = String(response.resumeToken || state.resumeToken || "");
    state.powerTouched = false;
    if (state.resumeToken) {
      localStorage.setItem(
        `ocean-hunters:resume:${state.roomId}`,
        JSON.stringify({ name: state.profile.name, token: state.resumeToken })
      );
    }
    if (response.state) setSnapshot(response.state, { force: true });

    ui.roomLabel.textContent = state.roomId;
    ui.roomButton.disabled = false;
    ui.joinOverlay.classList.add("hidden");
    ui.reconnectMask.hidden = true;
    setJoining(false);
    setConnectionStatus("online", "实时在线");
  });
}

function changePower(delta) {
  if (!state.joined) return;
  const currentIndex = closestPowerIndex(state.selectedPower);
  const nextIndex = clamp(currentIndex + delta, 0, POWER_LEVELS.length - 1);
  const nextPower = POWER_LEVELS[nextIndex];
  if (nextPower === state.selectedPower) return;
  state.selectedPower = nextPower;
  state.powerTouched = true;
  socket.emit("player:power", { power: nextPower });
  updateHud();
}

function rayExitPoint(origin, angle) {
  const dx = Math.cos(angle);
  const dy = Math.sin(angle);
  const candidates = [];
  if (Math.abs(dx) > 0.0001) {
    candidates.push((0 - origin.x) / dx, (WORLD.width - origin.x) / dx);
  }
  if (Math.abs(dy) > 0.0001) {
    candidates.push((0 - origin.y) / dy, (WORLD.height - origin.y) / dy);
  }
  const positive = candidates.filter((value) => value > 0);
  const distance = positive.length ? Math.min(...positive) : 700;
  return { x: origin.x + dx * distance, y: origin.y + dy * distance };
}

function visualAimAngle(player) {
  if (String(player?.id) === String(state.playerId) && state.lockedFishId) {
    const locked = fishesFrom().find((fish) => String(fish.id) === state.lockedFishId);
    if (locked) {
      const origin = displaySeatOrigin(player?.seat);
      const target = toViewPoint({ x: Number(locked.x), y: Number(locked.y) });
      return Math.atan2(target.y - origin.y, target.x - origin.x);
    }
  }
  const remembered = state.visualAimAngles.get(String(player?.id));
  if (Number.isFinite(remembered)) return remembered;
  const displayOrigin = displaySeatOrigin(player?.seat);
  const physicsOrigin = physicsSeatOrigin(player);
  const physicsAngle = Number.isFinite(player?.angle) ? player.angle : canonicalSeatOrigin(player?.seat).inward;
  const proxy = rayExitPoint(physicsOrigin, physicsAngle);
  const displayProxy = toViewPoint(proxy);
  return Math.atan2(displayProxy.y - displayOrigin.y, displayProxy.x - displayOrigin.x);
}

function activeTargetPoint(fallback = state.pointer) {
  if (state.lockedFishId) {
    const fish = fishesFrom().find((item) => String(item.id) === state.lockedFishId);
    if (fish) return { x: Number(fish.x), y: Number(fish.y) };
  }
  return fallback;
}

function aimAt(point, force = false) {
  const me = currentPlayer();
  if (!me || !state.joined) return null;
  const target = activeTargetPoint(point);
  const physicsOrigin = physicsSeatOrigin(me);
  const displayOrigin = displaySeatOrigin(me.seat);
  const displayTarget = toViewPoint(target);
  const serverAngle = Math.atan2(target.y - physicsOrigin.y, target.x - physicsOrigin.x);
  const displayAngle = Math.atan2(displayTarget.y - displayOrigin.y, displayTarget.x - displayOrigin.x);
  state.visualAimAngles.set(String(me.id), displayAngle);
  const now = performance.now();
  if (force || now - state.lastAimSentAt > 45) {
    state.lastAimSentAt = now;
    socket.emit("player:aim", { angle: serverAngle });
  }
  me.angle = serverAngle;
  return { angle: serverAngle, displayAngle, origin: displayOrigin, target };
}

function fireAt(point = state.pointer) {
  if (!state.joined || !socket.connected) return;
  const now = performance.now();
  if (now - state.lastLocalFireAt < 118) return;
  const me = currentPlayer();
  if (!me) return;
  if (Number(me.score || 0) < state.selectedPower) {
    if (!state.autoFire) showToast("积分不足，无法开炮");
    return;
  }

  const aim = aimAt(point, true);
  if (!aim) return;
  state.lastLocalFireAt = now;
  const commandId = crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  state.muzzleFlashes.push({
    x: aim.origin.x,
    y: aim.origin.y,
    angle: aim.displayAngle,
    power: state.selectedPower,
    color: playerColor(me),
    bornAt: now
  });
  socket.timeout(2200).emit("player:fire", { commandId, angle: aim.angle }, (timeoutError, response) => {
    if (timeoutError || response?.ok !== false) return;
    const code = response.error?.code;
    if (state.autoFire && code === "RATE_LIMITED") return;
    showToast(response.error?.message || response.error || "开炮失败");
  });
}

function syncAutoTimer() {
  clearInterval(state.autoTimer);
  state.autoTimer = 0;
  if (state.autoFire) {
    state.autoTimer = window.setInterval(() => {
      if (!document.hidden && state.joined && socket.connected) fireAt(activeTargetPoint());
    }, 150);
  }
  updateHud();
}

function toggleAutoFire() {
  if (!state.joined) return;
  state.autoFire = !state.autoFire;
  syncAutoTimer();
  showToast(state.autoFire ? "自动开炮已开启" : "自动开炮已关闭");
}

function nearestFish(point, maximumDistance = 92) {
  let nearest = null;
  let best = maximumDistance * maximumDistance;
  for (const fish of fishesFrom()) {
    const dx = Number(fish.x) - point.x;
    const dy = Number(fish.y) - point.y;
    const distance = dx * dx + dy * dy;
    const allowance = Math.max(maximumDistance, Number(fish.radius || 25) * Number(fish.scale || 1) * 1.35);
    if (distance <= allowance * allowance && distance < best) {
      nearest = fish;
      best = distance;
    }
  }
  return nearest;
}

function toggleLock() {
  if (!state.joined) return;
  if (state.lockedFishId || state.lockSeeking) {
    state.lockedFishId = "";
    state.lockSeeking = false;
    updateHud();
    showToast("目标锁定已取消");
    return;
  }
  const target = nearestFish(state.pointer);
  if (target) {
    state.lockedFishId = String(target.id);
    showToast(`已锁定 ${fishSpec(target).name}`);
  } else {
    state.lockSeeking = true;
    showToast("点击一条鱼完成锁定");
  }
  updateHud();
}

function pointerToWorld(event) {
  const rect = canvas.getBoundingClientRect();
  let viewPoint;
  if (PORTRAIT_LANDSCAPE_QUERY.matches) {
    viewPoint = {
      x: clamp((event.clientY - rect.top) * WORLD.width / rect.height, 0, WORLD.width),
      y: clamp((rect.right - event.clientX) * WORLD.height / rect.width, 0, WORLD.height)
    };
  } else {
    viewPoint = {
      x: clamp((event.clientX - rect.left) * WORLD.width / rect.width, 0, WORLD.width),
      y: clamp((event.clientY - rect.top) * WORLD.height / rect.height, 0, WORLD.height)
    };
  }
  return toWorldPoint(viewPoint);
}

function updateCrosshair(event, visible = true) {
  const frame = ui.gameFrame.getBoundingClientRect();
  if (PORTRAIT_LANDSCAPE_QUERY.matches) {
    const localX = (event.clientY - frame.top) * ui.gameFrame.clientWidth / frame.height;
    const localY = (frame.right - event.clientX) * ui.gameFrame.clientHeight / frame.width;
    ui.crosshair.style.left = `${clamp(localX, 0, ui.gameFrame.clientWidth)}px`;
    ui.crosshair.style.top = `${clamp(localY, 0, ui.gameFrame.clientHeight)}px`;
  } else {
    ui.crosshair.style.left = `${event.clientX - frame.left}px`;
    ui.crosshair.style.top = `${event.clientY - frame.top}px`;
  }
  ui.crosshair.classList.toggle("visible", visible && state.joined && !state.lockedFishId);
}

function roundedRect(context, x, y, width, height, radius) {
  const r = Math.min(radius, width / 2, height / 2);
  context.beginPath();
  context.moveTo(x + r, y);
  context.arcTo(x + width, y, x + width, y + height, r);
  context.arcTo(x + width, y + height, x, y + height, r);
  context.arcTo(x, y + height, x, y, r);
  context.arcTo(x, y, x + width, y, r);
  context.closePath();
}

function drawImageCover(image, x, y, width, height) {
  if (!image.complete || !image.naturalWidth) return false;
  const sourceRatio = image.naturalWidth / image.naturalHeight;
  const targetRatio = width / height;
  let sourceX = 0;
  let sourceY = 0;
  let sourceWidth = image.naturalWidth;
  let sourceHeight = image.naturalHeight;
  if (sourceRatio > targetRatio) {
    sourceWidth = image.naturalHeight * targetRatio;
    sourceX = (image.naturalWidth - sourceWidth) / 2;
  } else {
    sourceHeight = image.naturalWidth / targetRatio;
    sourceY = (image.naturalHeight - sourceHeight) / 2;
  }
  ctx.drawImage(image, sourceX, sourceY, sourceWidth, sourceHeight, x, y, width, height);
  return true;
}

function drawBackground(now) {
  if (!drawImageCover(images.arena, 0, 0, WORLD.width, WORLD.height)) {
    const fallback = ctx.createLinearGradient(0, 0, 0, WORLD.height);
    fallback.addColorStop(0, "#073d58");
    fallback.addColorStop(0.48, "#05273c");
    fallback.addColorStop(1, "#01111d");
    ctx.fillStyle = fallback;
    ctx.fillRect(0, 0, WORLD.width, WORLD.height);
  }

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.globalAlpha = 0.15;
  for (let index = 0; index < 5; index += 1) {
    const center = 125 + index * 275 + Math.sin(now * 0.00017 + index * 1.7) * 45;
    const beam = ctx.createLinearGradient(center - 90, 0, center + 130, WORLD.height * 0.76);
    beam.addColorStop(0, "rgba(169,249,240,0.42)");
    beam.addColorStop(1, "rgba(67,174,190,0)");
    ctx.fillStyle = beam;
    ctx.beginPath();
    ctx.moveTo(center - 50, 0);
    ctx.lineTo(center + 56, 0);
    ctx.lineTo(center + 230, WORLD.height * 0.78);
    ctx.lineTo(center + 35, WORLD.height * 0.78);
    ctx.closePath();
    ctx.fill();
  }
  ctx.restore();

  for (const particle of particles) {
    const y = (particle.y + now * 0.001 * particle.speed) % WORLD.height;
    const x = particle.x + Math.sin(now * 0.00045 + particle.phase) * particle.drift;
    ctx.fillStyle = `rgba(197,236,230,${particle.alpha})`;
    ctx.beginPath();
    ctx.arc(x, y, particle.radius, 0, Math.PI * 2);
    ctx.fill();
  }

  for (const bubble of bubbles) {
    const travel = (now * 0.001 * bubble.speed + bubble.y) % (WORLD.height + 70);
    const y = WORLD.height + 25 - travel;
    const x = bubble.x + Math.sin(now * 0.0007 + bubble.phase) * 14;
    ctx.strokeStyle = `rgba(190,244,241,${0.1 + bubble.radius * 0.018})`;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.arc(x, y, bubble.radius, 0, Math.PI * 2);
    ctx.stroke();
  }

  const depthFog = ctx.createLinearGradient(0, 0, 0, WORLD.height);
  depthFog.addColorStop(0, "rgba(0,40,54,0)");
  depthFog.addColorStop(0.72, "rgba(0,28,43,0.03)");
  depthFog.addColorStop(1, "rgba(0,14,30,0.14)");
  ctx.fillStyle = depthFog;
  ctx.fillRect(0, 0, WORLD.width, WORLD.height);
}

function interpolateEntities(kind) {
  const current = kind === "fish" ? fishesFrom(state.current) : bulletsFrom(state.current);
  const previous = kind === "fish" ? fishesFrom(state.previous) : bulletsFrom(state.previous);
  const previousById = new Map(previous.map((entity) => [String(entity.id), entity]));
  const alpha = clamp((performance.now() - state.snapshotAt) / 100, 0, 1);
  return current.map((entity) => {
    const before = previousById.get(String(entity.id));
    if (!before) return entity;
    const beforeAngle = Number(before.angle || 0);
    const angle = beforeAngle + shortestAngleDelta(Number(entity.angle || 0), beforeAngle) * alpha;
    let visualPitch = 0;
    if (kind === "fish" && !REDUCED_MOTION_QUERY.matches) {
      const deltaX = Number(entity.x || 0) - Number(before.x || 0);
      const deltaY = Number(entity.y || 0) - Number(before.y || 0);
      if (Math.abs(deltaX) + Math.abs(deltaY) > 0.2) {
        const trajectoryAngle = Math.atan2(deltaY, deltaX);
        visualPitch = clamp(shortestAngleDelta(trajectoryAngle, angle), -0.16, 0.16);
      }
    }
    return {
      ...entity,
      x: lerp(Number(before.x || 0), Number(entity.x || 0), alpha),
      y: lerp(Number(before.y || 0), Number(entity.y || 0), alpha),
      angle,
      visualPitch
    };
  });
}

function drawAnimatedFishSprite(spec, x, y, size, angle, options = {}) {
  const image = images[spec.asset];
  if (!image?.complete || !image.naturalWidth) return false;
  const frameDuration = 1000 / spec.fps;
  const phaseOffset = (options.motion?.phase || 0) / (Math.PI * 2) * spec.swimFrames;
  const frameIndex = options.reducedMotion
    ? 0
    : Math.floor(options.now / frameDuration + phaseOffset) % spec.swimFrames;
  const sourceY = frameIndex * spec.frameHeight;
  const drawWidth = size;
  const drawHeight = size * spec.frameHeight / spec.frameWidth;

  ctx.save();
  ctx.translate(x, y);
  // 素材包中的鱼默认朝右；向左游时镜像，保持鱼腹方向正确。
  const facingRight = Math.cos(angle) >= 0;
  const localPitch = shortestAngleDelta(angle, facingRight ? 0 : Math.PI);
  ctx.rotate(localPitch);
  if (!facingRight) ctx.scale(-1, 1);
  ctx.globalAlpha = options.alpha ?? 1;
  ctx.filter = options.filter || "none";
  ctx.shadowColor = options.shadowColor || "rgba(0,5,12,0.72)";
  ctx.shadowBlur = options.shadowBlur ?? 18;
  ctx.shadowOffsetY = options.shadowOffsetY ?? 8;
  ctx.drawImage(
    image,
    0,
    sourceY,
    spec.frameWidth,
    spec.frameHeight,
    -drawWidth / 2,
    -drawHeight / 2,
    drawWidth,
    drawHeight
  );
  ctx.restore();
  return true;
}

function drawFish(fish, now) {
  const spec = fishSpec(fish);
  const motion = fishMotionFor(fish, spec, now);
  const reducedMotion = REDUCED_MOTION_QUERY.matches;
  const radius = Number(fish.radius || (spec.boss ? 47 : 28));
  const serverScale = clamp(Number(fish.scale || 1), 0.58, 2.1);
  const size = clamp(Math.max(84, radius * 3.2 * serverScale) * spec.size, 78, spec.boss ? 265 : 190);
  const worldX = Number(fish.x || 0);
  const floatOffset = reducedMotion
    ? 0
    : Math.sin(now * 0.0024 * motion.rateScale + motion.phase * 1.31) * spec.drift;
  const worldY = Number(fish.y || 0) + floatOffset;
  const viewPoint = toViewPoint({ x: worldX, y: worldY });
  const x = viewPoint.x;
  const y = viewPoint.y;
  const elapsed = clamp(now - motion.updatedAt, 0, 50);
  const pitchLimit = spec.boss ? 0.075 : 0.145;
  const targetPitch = reducedMotion ? 0 : clamp(Number(fish.visualPitch || 0), -pitchLimit, pitchLimit);
  const pitchBlend = 1 - Math.exp(-elapsed / 135);
  motion.pitch += (targetPitch - motion.pitch) * pitchBlend;
  motion.updatedAt = now;
  const angle = toViewAngle(Number(fish.angle || 0) + motion.pitch);
  const flashingUntil = state.fishFlashes.get(String(fish.id)) || 0;
  const flashing = flashingUntil > now;

  if (spec.boss) {
    ctx.save();
    ctx.translate(x, y);
    ctx.rotate(reducedMotion ? 0 : now * 0.00023);
    ctx.setLineDash([9, 12]);
    ctx.strokeStyle = "rgba(246,191,78,0.35)";
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(0, 0, size * 0.42 + (reducedMotion ? 0 : Math.sin(now * 0.003) * 3), 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();
  }

  const drawn = drawAnimatedFishSprite(spec, x, y, size, angle, {
    now,
    motion,
    reducedMotion,
    filter: flashing ? "brightness(3) saturate(0)" : "none",
    shadowColor: spec.boss ? "rgba(1,2,7,0.88)" : "rgba(0,5,12,0.68)",
    shadowBlur: spec.boss ? 28 : 17
  });

  if (!drawn) {
    ctx.save();
    ctx.translate(x, y);
    ctx.rotate(angle);
    ctx.fillStyle = flashing ? "#ffffff" : (spec.boss ? "#536c78" : "#4baab7");
    ctx.beginPath();
    ctx.ellipse(0, 0, size * 0.34, size * 0.19, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  const multiplier = fishMultiplier(fish);
  const labelY = y - size * (spec.boss ? 0.36 : 0.32);
  const width = spec.boss ? 84 : 66;
  roundedRect(ctx, x - width / 2, labelY - 10, width, 21, 8);
  const labelGradient = ctx.createLinearGradient(x - width / 2, labelY, x + width / 2, labelY);
  labelGradient.addColorStop(0, "rgba(4,14,20,0.76)");
  labelGradient.addColorStop(0.5, spec.boss ? "rgba(79,50,12,0.87)" : "rgba(4,30,37,0.86)");
  labelGradient.addColorStop(1, "rgba(4,14,20,0.76)");
  ctx.fillStyle = labelGradient;
  ctx.fill();
  ctx.strokeStyle = spec.boss ? "rgba(246,193,79,0.6)" : "rgba(94,224,229,0.38)";
  ctx.lineWidth = 1;
  ctx.stroke();
  ctx.fillStyle = spec.boss ? "#ffe394" : "#e9feff";
  ctx.font = `900 ${spec.boss ? 13 : 11}px system-ui`;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText(`×${multiplier}`, x, labelY + 0.5);

  ctx.fillStyle = spec.boss ? "rgba(255,225,151,0.86)" : "rgba(203,229,232,0.72)";
  ctx.font = `700 ${spec.boss ? 9 : 8}px system-ui`;
  ctx.fillText(spec.name, x, labelY + 18);

  if (String(fish.id) === state.lockedFishId) drawTargetLock(x, y, size * 0.42, now);
}

function drawTargetLock(x, y, radius, now) {
  ctx.save();
  ctx.translate(x, y);
  ctx.rotate(now * 0.0012);
  ctx.strokeStyle = "rgba(255,220,105,0.94)";
  ctx.lineWidth = 2;
  for (let corner = 0; corner < 4; corner += 1) {
    ctx.rotate(Math.PI / 2);
    ctx.beginPath();
    ctx.moveTo(radius - 15, -radius);
    ctx.lineTo(radius, -radius);
    ctx.lineTo(radius, -radius + 15);
    ctx.stroke();
  }
  ctx.setLineDash([5, 7]);
  ctx.beginPath();
  ctx.arc(0, 0, radius * 0.72, 0, Math.PI * 2);
  ctx.stroke();
  ctx.restore();
}

function visualBulletPoint(bullet, owner) {
  const bulletPoint = { x: Number(bullet.x || 0), y: Number(bullet.y || 0) };
  if (!owner) return toViewPoint(bulletPoint);
  const physicsOrigin = physicsSeatOrigin(owner);
  const layout = String(state.current?.seatLayout || state.current?.layout || "").toLowerCase();
  if (layout.includes("four") || layout.includes("table") || (Number.isFinite(owner.x) && Number.isFinite(owner.y))) {
    return toViewPoint(bulletPoint);
  }
  const canonicalOrigin = canonicalSeatOrigin(owner.seat);
  const travel = Math.hypot(bulletPoint.x - physicsOrigin.x, bulletPoint.y - physicsOrigin.y);
  const physicsAngle = Number.isFinite(owner?.angle) ? owner.angle : canonicalOrigin.inward;
  const proxy = rayExitPoint(physicsOrigin, physicsAngle);
  const canonicalAngle = Math.atan2(proxy.y - canonicalOrigin.y, proxy.x - canonicalOrigin.x);
  return toViewPoint({
    x: canonicalOrigin.x + Math.cos(canonicalAngle) * travel,
    y: canonicalOrigin.y + Math.sin(canonicalAngle) * travel
  });
}

function drawBullet(bullet) {
  const owner = playersFrom().find((player) => String(player.id) === String(bullet.ownerId || bullet.playerId));
  const color = playerColor(owner);
  const point = visualBulletPoint(bullet, owner);
  const id = String(bullet.id);
  const previous = state.bulletHistory.get(id);
  if (previous) {
    const gradient = ctx.createLinearGradient(previous.x, previous.y, point.x, point.y);
    gradient.addColorStop(0, "rgba(255,255,255,0)");
    gradient.addColorStop(0.62, color);
    gradient.addColorStop(1, "#f8ffff");
    ctx.strokeStyle = gradient;
    ctx.lineWidth = 2.2 + Math.min(5, Number(bullet.power || 1)) * 0.75;
    ctx.beginPath();
    ctx.moveTo(previous.x, previous.y);
    ctx.lineTo(point.x, point.y);
    ctx.stroke();
  }
  state.bulletHistory.set(id, { x: point.x, y: point.y, seenAt: performance.now() });

  ctx.save();
  ctx.shadowColor = color;
  ctx.shadowBlur = 18;
  ctx.fillStyle = "#f7ffff";
  ctx.beginPath();
  ctx.arc(point.x, point.y, 3.5 + Math.min(5, Number(bullet.power || 1)) * 0.58, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function cannonPowerFor(player) {
  if (String(player?.id) === String(state.playerId)) return state.selectedPower;
  return Math.max(1, Number(player?.power || 1));
}

function drawCannon(seat, player, now) {
  const origin = displaySeatOrigin(seat);
  const color = playerColor(player || { seat });
  const occupied = Boolean(player);
  const mine = occupied && String(player.id) === String(state.playerId);
  const angle = occupied ? visualAimAngle(player) : origin.inward;
  const powerIndex = closestPowerIndex(cannonPowerFor(player));
  const palette = CANNON_PALETTES[powerIndex];
  const pulse = 1 + Math.sin(now * 0.004 + seat) * 0.04;

  ctx.save();
  ctx.translate(origin.x, origin.y);
  if (mine) {
    const halo = ctx.createRadialGradient(0, 0, 15, 0, 0, 72);
    halo.addColorStop(0, `${color}44`);
    halo.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = halo;
    ctx.beginPath();
    ctx.arc(0, 0, 74 * pulse, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = `${color}8f`;
    ctx.lineWidth = 2;
    ctx.setLineDash([8, 7]);
    ctx.beginPath();
    ctx.arc(0, 0, 54 * pulse, 0, Math.PI * 2);
    ctx.stroke();
    ctx.setLineDash([]);
  }

  ctx.globalAlpha = occupied ? 1 : 0.48;
  const mount = ctx.createRadialGradient(-10, -12, 3, 0, 0, 51);
  mount.addColorStop(0, occupied ? palette.light : "#87979c");
  mount.addColorStop(0.16, occupied ? palette.body : "#65757a");
  mount.addColorStop(0.55, occupied ? palette.dark : "#34454a");
  mount.addColorStop(0.82, "#163143");
  mount.addColorStop(1, "#061824");
  ctx.fillStyle = mount;
  ctx.beginPath();
  ctx.arc(0, 0, 45, 0, Math.PI * 2);
  ctx.fill();
  ctx.strokeStyle = occupied ? palette.trim : "rgba(176,197,202,0.45)";
  ctx.lineWidth = 3;
  ctx.stroke();

  ctx.save();
  ctx.rotate(now * 0.00016 * (seat % 2 ? -1 : 1));
  ctx.fillStyle = occupied ? palette.trim : "#81949b";
  for (let fin = 0; fin < 8; fin += 1) {
    ctx.rotate(Math.PI / 4);
    roundedRect(ctx, 31, -3, 11, 6, 3);
    ctx.fill();
  }
  ctx.restore();

  ctx.save();
  ctx.rotate(angle);
  const barrelLength = 58 + powerIndex * 5;
  const barrelWidth = 16 + powerIndex * 1.5;
  const barrelGradient = ctx.createLinearGradient(0, -barrelWidth / 2, 0, barrelWidth / 2);
  barrelGradient.addColorStop(0, occupied ? palette.light : "#bdc9cc");
  barrelGradient.addColorStop(0.18, occupied ? palette.trim : "#718188");
  barrelGradient.addColorStop(0.48, occupied ? palette.body : "#52646b");
  barrelGradient.addColorStop(0.82, occupied ? palette.dark : "#283940");
  barrelGradient.addColorStop(1, "#0b2432");
  roundedRect(ctx, 2, -barrelWidth / 2, barrelLength, barrelWidth, barrelWidth / 2);
  ctx.fillStyle = barrelGradient;
  ctx.fill();
  ctx.strokeStyle = occupied ? palette.trim : "rgba(137,163,171,0.48)";
  ctx.lineWidth = 2;
  ctx.stroke();

  roundedRect(ctx, barrelLength - 8, -barrelWidth * 0.72, 18, barrelWidth * 1.44, 5);
  ctx.fillStyle = occupied ? palette.body : "#52636a";
  ctx.fill();
  ctx.strokeStyle = occupied ? palette.trim : "rgba(229,250,250,0.34)";
  ctx.stroke();

  for (let band = 0; band < 2; band += 1) {
    roundedRect(ctx, 18 + band * 18, -barrelWidth * 0.58, 5, barrelWidth * 1.16, 2);
    ctx.fillStyle = occupied ? palette.dark : "rgba(2,11,16,0.48)";
    ctx.fill();
  }
  ctx.restore();

  ctx.fillStyle = occupied ? palette.dark : "#23363e";
  ctx.beginPath();
  ctx.arc(0, 0, 25, 0, Math.PI * 2);
  ctx.fill();
  ctx.strokeStyle = occupied ? palette.trim : "#65777e";
  ctx.lineWidth = 3;
  ctx.stroke();
  ctx.fillStyle = occupied ? palette.body : "#596b72";
  ctx.beginPath();
  ctx.arc(0, 0, 10, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = occupied ? "#ecffff" : "#8ba0a8";
  ctx.font = "900 9px system-ui";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText(`P${seat + 1}`, 0, 1);
  ctx.restore();
}

function spawnNetEffect(x, y, fishId = "", color = "#76edf0", power = 1) {
  const now = performance.now();
  const point = toViewPoint({ x, y });
  if (fishId) state.fishFlashes.set(String(fishId), now + 165);
  state.effects.push({
    type: "net",
    x: point.x,
    y: point.y,
    color,
    power: Math.max(1, Number(power) || 1),
    bornAt: now,
    duration: 620
  });
}

function spawnCatchEffect(event = {}) {
  const embeddedFish = event.fish || {};
  const fish = {
    ...embeddedFish,
    assetKey: event.assetKey ?? embeddedFish.assetKey,
    type: event.type ?? embeddedFish.type,
    multiplier: event.multiplier ?? embeddedFish.multiplier
  };
  const spec = fishSpec(fish);
  const multiplier = fishMultiplier(fish);
  const worldX = Number(event.x ?? embeddedFish.x ?? WORLD.width / 2);
  const worldY = Number(event.y ?? embeddedFish.y ?? WORLD.height / 2);
  const point = toViewPoint({ x: worldX, y: worldY });
  const x = point.x;
  const y = point.y;
  const reward = Math.max(0, Math.floor(Number(event.reward ?? event.payout ?? embeddedFish.reward ?? 0)));
  const player = playersFrom().find((item) => String(item.id) === String(event.playerId || event.ownerId));
  const target = player ? displaySeatOrigin(player.seat) : { x, y: WORLD.height + 40 };
  const coinCount = multiplier >= 40 ? 26 : multiplier >= 12 ? 22 : 18;
  const coinSeeds = Array.from({ length: coinCount }, (_, index) => ({
    angle: (Math.PI * 2 * index) / coinCount + ((index * 17) % 7) * 0.06,
    speed: 62 + ((index * 37) % 105),
    lift: 44 + ((index * 23) % 78),
    spin: 2 + ((index * 11) % 7),
    delay: (index % 4) * 0.035
  }));
  spawnNetEffect(worldX, worldY, String(event.fishId || embeddedFish.id || ""), "#ffe169", event.power || event.bet);
  state.effects.push({
    type: "catch",
    x,
    y,
    targetX: target.x,
    targetY: target.y,
    reward,
    multiplier,
    spec,
    color: "#ffd66c",
    bornAt: performance.now(),
    duration: 1350,
    coinSeeds
  });
  return { spec, multiplier, reward, x, y };
}

function drawNetEffect(effect, now) {
  const progress = clamp((now - effect.bornAt) / effect.duration, 0, 1);
  const powerTier = closestPowerIndex(effect.power);
  const targetRadius = 54 + powerTier * 10;
  const radius = 10 + Math.sin(progress * Math.PI * 0.72) * targetRadius;
  ctx.save();
  ctx.translate(effect.x, effect.y);
  ctx.rotate(progress * 0.24);
  ctx.globalAlpha = 1 - progress;
  ctx.strokeStyle = effect.color;
  ctx.lineWidth = 2.6 - progress * 1.5;
  ctx.shadowColor = effect.color;
  ctx.shadowBlur = 13;
  for (let ring = 0.25; ring <= 1; ring += 0.19) {
    ctx.beginPath();
    const points = 12;
    for (let point = 0; point <= points; point += 1) {
      const angle = point / points * Math.PI * 2;
      const ringRadius = radius * ring;
      const px = Math.cos(angle) * ringRadius;
      const py = Math.sin(angle) * ringRadius;
      if (point === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();
  }
  for (let spoke = 0; spoke < 12; spoke += 1) {
    const angle = spoke * Math.PI / 6;
    ctx.beginPath();
    ctx.moveTo(Math.cos(angle) * radius * 0.08, Math.sin(angle) * radius * 0.08);
    ctx.lineTo(Math.cos(angle) * radius, Math.sin(angle) * radius);
    ctx.stroke();
  }
  ctx.restore();
}

function drawCatchEffect(effect, now) {
  const progress = clamp((now - effect.bornAt) / effect.duration, 0, 1);
  ctx.save();
  ctx.translate(effect.x, effect.y);
  ctx.globalAlpha = 1 - progress;
  ctx.strokeStyle = "#ffd86d";
  ctx.lineWidth = 5 * (1 - progress) + 1;
  ctx.shadowColor = "#ffbd36";
  ctx.shadowBlur = 28;
  ctx.beginPath();
  ctx.arc(0, 0, 24 + progress * 84, 0, Math.PI * 2);
  ctx.stroke();
  ctx.setLineDash([9, 10]);
  ctx.beginPath();
  ctx.arc(0, 0, 15 + progress * 57, -progress * 2, Math.PI * 2 - progress * 2);
  ctx.stroke();
  ctx.setLineDash([]);
  ctx.restore();

  for (const coin of effect.coinSeeds) {
    const local = clamp(progress - coin.delay, 0, 1);
    if (local <= 0) continue;
    const distance = coin.speed * local;
    const burstX = effect.x + Math.cos(coin.angle) * distance;
    const burstY = effect.y + Math.sin(coin.angle) * distance - coin.lift * Math.sin(local * Math.PI) + local * local * 34;
    const recovery = clamp((local - 0.48) / 0.52, 0, 1);
    const recoveryEase = recovery * recovery * (3 - 2 * recovery);
    const x = lerp(burstX, effect.targetX, recoveryEase);
    const y = lerp(burstY, effect.targetY, recoveryEase) - Math.sin(recovery * Math.PI) * 26;
    ctx.save();
    ctx.globalAlpha = local < 0.9 ? 0.96 : (1 - local) * 9.6;
    ctx.translate(x, y);
    ctx.scale(Math.abs(Math.cos(local * Math.PI * coin.spin)), 1);
    ctx.fillStyle = "#ffd662";
    ctx.strokeStyle = "#fff0a9";
    ctx.lineWidth = 1;
    ctx.shadowColor = "#ffb52e";
    ctx.shadowBlur = 10;
    ctx.beginPath();
    ctx.arc(0, 0, 5.5, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.restore();
  }

  const rise = Math.sin(Math.min(1, progress * 1.7) * Math.PI * 0.5) * 55;
  ctx.save();
  ctx.globalAlpha = progress < 0.72 ? 1 : 1 - (progress - 0.72) / 0.28;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.shadowColor = "rgba(0,0,0,0.85)";
  ctx.shadowBlur = 10;
  ctx.lineWidth = 6;
  ctx.strokeStyle = "rgba(58,29,3,0.84)";
  ctx.font = "950 34px system-ui";
  ctx.strokeText(`×${effect.multiplier}`, effect.x, effect.y - 34 - rise);
  ctx.fillStyle = "#ffe37d";
  ctx.fillText(`×${effect.multiplier}`, effect.x, effect.y - 34 - rise);
  ctx.font = "800 11px system-ui";
  ctx.fillStyle = "#fff7cf";
  ctx.fillText(effect.spec.name, effect.x, effect.y - 10 - rise);
  if (effect.reward > 0) {
    ctx.font = "800 13px system-ui";
    ctx.fillStyle = "#ffffff";
    ctx.fillText(`+${effect.reward.toLocaleString("zh-CN")}`, effect.x, effect.y + 9 - rise);
  }
  ctx.restore();
}

function drawEffects(now) {
  state.muzzleFlashes = state.muzzleFlashes.filter((flash) => now - flash.bornAt < 210);
  for (const flash of state.muzzleFlashes) {
    const progress = (now - flash.bornAt) / 210;
    const distance = 72 + closestPowerIndex(flash.power) * 5;
    ctx.save();
    ctx.translate(flash.x + Math.cos(flash.angle) * distance, flash.y + Math.sin(flash.angle) * distance);
    ctx.globalAlpha = 1 - progress;
    ctx.rotate(flash.angle);
    ctx.fillStyle = "#fff6ba";
    ctx.shadowColor = flash.color;
    ctx.shadowBlur = 30;
    ctx.beginPath();
    ctx.moveTo(25 + progress * 22, 0);
    ctx.lineTo(-5, -9 - progress * 7);
    ctx.lineTo(3, 0);
    ctx.lineTo(-5, 9 + progress * 7);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  state.effects = state.effects.filter((effect) => now - effect.bornAt < effect.duration);
  for (const effect of state.effects) {
    if (effect.type === "catch") drawCatchEffect(effect, now);
    else drawNetEffect(effect, now);
  }

  for (const [id, expiresAt] of state.fishFlashes) {
    if (expiresAt <= now) state.fishFlashes.delete(id);
  }
  for (const [id, point] of state.bulletHistory) {
    if (now - point.seenAt > 500) state.bulletHistory.delete(id);
  }
  if (now - state.lastFishMotionSweepAt > 2000) {
    for (const [id, motion] of state.fishMotion) {
      if (now - motion.lastSeenAt > 2000) state.fishMotion.delete(id);
    }
    state.lastFishMotionSweepAt = now;
  }
}

let lastRenderAt = 0;

function render(now) {
  requestAnimationFrame(render);
  const elapsed = now - lastRenderAt;
  if (elapsed < RENDER_FRAME_INTERVAL) return;
  lastRenderAt = now - (elapsed % RENDER_FRAME_INTERVAL);

  drawBackground(now);
  for (const fish of interpolateEntities("fish")) drawFish(fish, now);
  for (const bullet of interpolateEntities("bullet")) drawBullet(bullet);
  const playersBySeat = new Map(playersFrom().map((player) => [Number(player.seat), player]));
  for (let seat = 0; seat < DISPLAY_SEATS.length; seat += 1) drawCannon(seat, playersBySeat.get(seat), now);
  drawEffects(now);

  const vignette = ctx.createRadialGradient(WORLD.width / 2, WORLD.height * 0.45, 210, WORLD.width / 2, WORLD.height * 0.45, 760);
  vignette.addColorStop(0, "rgba(0,0,0,0)");
  vignette.addColorStop(0.72, "rgba(0,9,18,0.025)");
  vignette.addColorStop(1, "rgba(0,8,18,0.27)");
  ctx.fillStyle = vignette;
  ctx.fillRect(0, 0, WORLD.width, WORLD.height);

  if (state.lockedFishId) {
    const locked = fishesFrom().find((fish) => String(fish.id) === state.lockedFishId);
    const me = currentPlayer();
    if (locked && me) {
      const origin = displaySeatOrigin(me.seat);
      const target = toViewPoint({ x: Number(locked.x), y: Number(locked.y) });
      ctx.save();
      ctx.strokeStyle = "rgba(255,218,99,0.24)";
      ctx.lineWidth = 1;
      ctx.setLineDash([5, 8]);
      ctx.beginPath();
      ctx.moveTo(origin.x, origin.y);
      ctx.lineTo(target.x, target.y);
      ctx.stroke();
      ctx.restore();
    }
  }
}

socket.on("connect", () => {
  setConnectionStatus("connecting", state.joined ? "恢复房间" : "连接中");
  if (state.profile) emitJoin();
});

socket.on("disconnect", () => {
  setConnectionStatus("offline", "已断线");
  if (state.joined) ui.reconnectMask.hidden = false;
});

socket.on("connect_error", (error) => {
  setJoining(false);
  setConnectionStatus("offline", "连接失败");
  if (!state.joined) showJoinError(`无法连接游戏服务：${error?.message || "未知错误"}`);
});

socket.on("game:snapshot", setSnapshot);

socket.on("game:catch", (event = {}) => {
  const player = playersFrom().find((item) => String(item.id) === String(event.playerId || event.ownerId));
  const result = spawnCatchEffect(event);
  const name = String(event.playerName || player?.name || "猎手");
  const rewardText = result.reward > 0 ? ` +${result.reward}` : "";
  addFeed(`${name} 捕获 ${result.spec.name} ×${result.multiplier}${rewardText}`, name);
});

function handleMissEvent(event = {}) {
  const fish = event.fish || fishesFrom().find((item) => String(item.id) === String(event.fishId)) || {};
  const x = Number(event.x ?? fish.x ?? WORLD.width / 2);
  const y = Number(event.y ?? fish.y ?? WORLD.height / 2);
  spawnNetEffect(
    x,
    y,
    String(event.fishId || fish.id || ""),
    event.color || "#75edf0",
    event.power || event.bet
  );
}

socket.on("game:hit", (event = {}) => {
  if (event.captured === false || event.success === false) handleMissEvent(event);
});
socket.on("game:miss", handleMissEvent);
socket.on("game:catch-failed", handleMissEvent);
socket.on("shot:resolved", (event = {}) => {
  // Captures also arrive through game:catch for backward compatibility; only
  // consume failed resolutions here so success effects are never duplicated.
  if (event.captured === false) handleMissEvent(event);
});

socket.on("room:notice", (notice) => {
  const message = typeof notice === "string" ? notice : notice?.message;
  if (message) addFeed(String(message));
});

socket.on("fatal", (payload) => {
  showToast(payload?.message || "房间连接已结束");
  resetToJoin("");
});

window.leaveFishingSession = function leaveFishingSession() {
  return new Promise((resolve) => {
    if (!socket.connected || !state.joined) {
      resolve();
      return;
    }
    socket.timeout(5000).emit("session:leave", {}, () => resolve());
  });
};

ui.joinForm.addEventListener("submit", (event) => {
  event.preventDefault();
  submitJoin();
});

ui.roomInput.addEventListener("input", () => {
  const normalized = normalizeRoomId(ui.roomInput.value);
  if (ui.roomInput.value !== normalized) ui.roomInput.value = normalized;
});

ui.randomRoom.addEventListener("click", () => {
  ui.roomInput.value = randomRoomId();
  showJoinError("");
});

ui.powerDown.addEventListener("click", () => changePower(-1));
ui.powerUp.addEventListener("click", () => changePower(1));
ui.lockButton.addEventListener("click", toggleLock);
ui.autoButton.addEventListener("click", toggleAutoFire);
ui.fireButton.addEventListener("pointerdown", (event) => {
  event.preventDefault();
  fireAt(activeTargetPoint());
});

ui.roomButton.addEventListener("click", () => {
  showToast(`当前自动匹配牌桌：${state.roomId}`);
});

canvas.addEventListener("pointermove", (event) => {
  state.pointer = { ...pointerToWorld(event), active: true };
  updateCrosshair(event, true);
  if (!state.lockedFishId) aimAt(state.pointer);
});

canvas.addEventListener("pointerenter", (event) => updateCrosshair(event, true));
canvas.addEventListener("pointerleave", () => ui.crosshair.classList.remove("visible"));
canvas.addEventListener("pointerdown", (event) => {
  event.preventDefault();
  state.pointer = { ...pointerToWorld(event), active: true };
  updateCrosshair(event, true);
  if (state.lockSeeking) {
    const target = nearestFish(state.pointer, 120);
    if (target) {
      state.lockedFishId = String(target.id);
      state.lockSeeking = false;
      showToast(`已锁定 ${fishSpec(target).name}`);
      updateHud();
    } else {
      showToast("这里没有可锁定目标");
      return;
    }
  }
  fireAt(activeTargetPoint(state.pointer));
});

window.addEventListener("keydown", (event) => {
  if (!state.joined || /INPUT|TEXTAREA/.test(document.activeElement?.tagName || "")) return;
  if (event.key === " " || event.key === "Enter") {
    event.preventDefault();
    fireAt(activeTargetPoint());
  } else if (event.key === "ArrowUp" || event.key === "+" || event.key === "=") {
    event.preventDefault();
    changePower(1);
  } else if (event.key === "ArrowDown" || event.key === "-") {
    event.preventDefault();
    changePower(-1);
  } else if (event.key.toLowerCase() === "a") {
    event.preventDefault();
    toggleAutoFire();
  } else if (event.key.toLowerCase() === "l") {
    event.preventDefault();
    toggleLock();
  }
});

document.addEventListener("visibilitychange", () => {
  if (!document.hidden && state.joined && !socket.connected) socket.connect();
});

const launchQuery = new URLSearchParams(location.search);
const launchTable = Number(launchQuery.get("table"));
const launchSeat = Number(launchQuery.get("seat"));
const launchSession = String(launchQuery.get("session") || "");
const launchResume = String(launchQuery.get("resume") || "");
const launchVenue = String(launchQuery.get("venue") || "novice");
const launchName = normalizeName(launchQuery.get("name"));
const hasLaunchTicket = Number.isInteger(launchTable) && launchTable >= 1 && launchTable <= 300
  && Number.isInteger(launchSeat) && launchSeat >= 1 && launchSeat <= 4
  && launchSession.length === 26 && launchResume.length >= 32;

if (hasLaunchTicket) {
  const matchedRoom = `${launchVenue.toUpperCase().slice(0, 3)}T${String(launchTable).padStart(3, "0")}`;
  state.profile = {
    name: launchName || "深海猎手",
    roomId: matchedRoom,
    session: launchSession,
    resume: launchResume,
    venue: launchVenue,
    table: launchTable,
    seat: launchSeat
  };
  state.localSeat = launchSeat - 1;
  state.roomId = matchedRoom;
  ui.nameInput.value = state.profile.name;
  ui.roomInput.value = matchedRoom;
  setJoining(true);
  setConnectionStatus("connecting", "自动匹配中");
  socket.connect();
} else {
  const queryRoom = normalizeRoomId(launchQuery.get("room"));
  ui.roomInput.value = queryRoom.length >= 4 ? queryRoom : randomRoomId();
  ui.nameInput.value = normalizeName(localStorage.getItem("ocean-hunters:name"));
  if (ui.nameInput.value) ui.roomInput.focus();
  else ui.nameInput.focus();
}

updateHud();
requestAnimationFrame(render);
