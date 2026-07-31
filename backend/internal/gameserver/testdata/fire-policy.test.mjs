import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

const source = await readFile(
  new URL("../fishingassets/fire-policy.js", import.meta.url),
  "utf8"
);
const gameSource = await readFile(
  new URL("../fishingassets/game.js", import.meta.url),
  "utf8"
);
const indexSource = await readFile(
  new URL("../fishingassets/index.html", import.meta.url),
  "utf8"
);
const policy = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`
);

const {
  DEFAULT_AUTO_FIRE_INTERVAL_MS,
  DEFAULT_FIRE_INTERVAL_MS,
  evaluateFireAttempt,
  fireFailureMessage,
  normalizePowerLevels,
  registerShotResolution,
  shouldShowFireFailure
} = policy;

test("entry and policy imports share a fresh cache version", () => {
  const entryVersion = indexSource.match(/game\.js\?v=([^"']+)/)?.[1];
  const policyVersion = gameSource.match(/fire-policy\.js\?v=([^"']+)/)?.[1];
  assert.equal(entryVersion, "20260731-physics1");
  assert.equal(policyVersion, entryVersion);
  assert.match(gameSource, /import \* as firePolicy/);
});

test("all seven paid projectile and net tiers have render assets", async () => {
  for (let tier = 1; tier <= 7; tier += 1) {
    await access(new URL(`../fishingassets/assets/bullet/bullet${tier}.png`, import.meta.url));
    await access(new URL(`../fishingassets/assets/net/web${tier}.png`, import.meta.url));
  }
});

test("venue cannon levels replace the legacy global list exactly", () => {
  assert.deepEqual(normalizePowerLevels([10, 20, 50, 100]), [10, 20, 50, 100]);
  assert.deepEqual(normalizePowerLevels([50, "5", 25, 10, 25]), [5, 10, 25, 50]);
  assert.deepEqual(normalizePowerLevels([], [1, 2, 5, 10]), [1, 2, 5, 10]);
});

test("client fire cadence stays below the sustained server token rate", () => {
  assert.ok(DEFAULT_FIRE_INTERVAL_MS >= 150);
  assert.ok(DEFAULT_AUTO_FIRE_INTERVAL_MS >= 180);
  assert.ok(DEFAULT_AUTO_FIRE_INTERVAL_MS >= DEFAULT_FIRE_INTERVAL_MS);

  const first = evaluateFireAttempt({}, { now: 1_000, inputToken: "pointer:1" });
  assert.equal(first.accepted, true);

  const tooSoon = evaluateFireAttempt(
    first.gate,
    { now: 1_000 + DEFAULT_FIRE_INTERVAL_MS - 1, inputToken: "pointer:2" }
  );
  assert.deepEqual(
    { accepted: tooSoon.accepted, reason: tooSoon.reason },
    { accepted: false, reason: "throttled" }
  );

  const boundary = evaluateFireAttempt(
    tooSoon.gate,
    { now: 1_000 + DEFAULT_FIRE_INTERVAL_MS, inputToken: "pointer:3" }
  );
  assert.equal(boundary.accepted, true);
});

test("a fresh gate with an explicit null timestamp accepts its first shot", () => {
  const first = evaluateFireAttempt(
    { lastAcceptedAt: null, recentInputTokens: [] },
    { now: 1, inputToken: "first-shot" }
  );
  assert.equal(first.accepted, true);
  assert.equal(first.gate.lastAcceptedAt, 1);
});

test("callers cannot lower the fire interval below the safe minimum", () => {
  const first = evaluateFireAttempt({}, { now: 500, inputToken: "key:1" }, 1);
  const second = evaluateFireAttempt(first.gate, { now: 649, inputToken: "key:2" }, 1);
  assert.equal(second.accepted, false);
  assert.equal(second.reason, "throttled");
});

test("one input token can emit at most one shot", () => {
  let gate = {};
  let emitted = 0;
  for (const now of [2_000, 2_001, 2_500]) {
    const result = evaluateFireAttempt(gate, {
      now,
      inputToken: "pointerdown:17:1999.5"
    });
    gate = result.gate;
    if (result.accepted) emitted += 1;
  }
  assert.equal(emitted, 1);
  assert.equal(gate.lastAcceptedAt, 2_000);
});

test("an earlier input token stays consumed after other inputs arrive", () => {
  let gate = {};
  for (const [now, inputToken] of [[2_000, "tap:a"], [2_200, "tap:b"]]) {
    gate = evaluateFireAttempt(gate, { now, inputToken }).gate;
  }
  const replay = evaluateFireAttempt(gate, { now: 2_400, inputToken: "tap:a" });
  assert.equal(replay.accepted, false);
  assert.equal(replay.reason, "duplicate-input");
});

test("a throttled input token is still consumed and cannot fire later", () => {
  const first = evaluateFireAttempt({}, { now: 10_000, inputToken: "tap:1" });
  const throttled = evaluateFireAttempt(first.gate, {
    now: 10_010,
    inputToken: "tap:2"
  });
  assert.equal(throttled.reason, "throttled");

  const replay = evaluateFireAttempt(throttled.gate, {
    now: 11_000,
    inputToken: "tap:2"
  });
  assert.equal(replay.accepted, false);
  assert.equal(replay.reason, "duplicate-input");
});

test("rate limiting and aim misses are non-fatal and never produce a toast", () => {
  for (const code of ["RATE_LIMITED", "NO_TARGET"]) {
    const response = {
      ok: false,
      error: { code, message: code === "RATE_LIMITED" ? "开炮速度过快" : "当前方向没有可捕获目标" }
    };
    assert.equal(fireFailureMessage(response), "");
    assert.equal(shouldShowFireFailure(response), false);
  }
});

test("actionable fire failures keep their server message and generic fallback", () => {
  const funds = {
    ok: false,
    error: { code: "INSUFFICIENT_FUNDS", message: "捕鱼托管余额不足" }
  };
  assert.equal(fireFailureMessage(funds), "捕鱼托管余额不足");
  assert.equal(shouldShowFireFailure(funds), true);
  assert.equal(fireFailureMessage({ ok: false, error: {} }), "开炮失败");
  assert.equal(fireFailureMessage({ ok: true }), "");
});

test("an expired session remains actionable so the client stops firing and asks for re-entry", () => {
  const expired = {
    ok: false,
    error: { code: "SESSION_EXPIRED", message: "捕鱼会话已失效，请重新进入" }
  };
  assert.equal(fireFailureMessage(expired), "捕鱼会话已失效，请重新进入");
  assert.equal(shouldShowFireFailure(expired), true);
});

test("an empty-water resolution needs no fish id to create exactly one net effect", () => {
  const seen = new Map();
  const event = {
    shotId: "empty-water-shot-22",
    captured: false,
    x: 430,
    y: 0,
    power: 5
  };
  assert.equal(registerShotResolution(seen, event, 200), true);
  assert.equal(registerShotResolution(seen, { ...event }, 201), false);
  assert.equal(seen.get(event.shotId), 200);
});

test("duplicate resolution broadcasts create only one miss effect", () => {
  const seen = new Map();
  const event = { shotId: "shot-17", captured: false };
  assert.equal(registerShotResolution(seen, event, 100), true);
  assert.equal(registerShotResolution(seen, event, 101), false);
  assert.equal(seen.size, 1);
});
