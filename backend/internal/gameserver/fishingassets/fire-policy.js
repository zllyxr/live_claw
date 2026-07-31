export const DEFAULT_FIRE_INTERVAL_MS = 150;
export const DEFAULT_AUTO_FIRE_INTERVAL_MS = 180;
export const DEFAULT_POWER_LEVELS = Object.freeze([1, 2, 5, 10, 20, 50]);

const SILENT_FIRE_FAILURE_CODES = new Set(["RATE_LIMITED", "NO_TARGET"]);

function finiteTimestamp(value, fallback = null) {
  if (value === null || value === undefined || value === "") return fallback;
  const timestamp = Number(value);
  return Number.isFinite(timestamp) ? timestamp : fallback;
}

function normalizeInterval(value) {
  const interval = finiteTimestamp(value, DEFAULT_FIRE_INTERVAL_MS);
  return interval >= DEFAULT_FIRE_INTERVAL_MS ? interval : DEFAULT_FIRE_INTERVAL_MS;
}

function normalizeInputToken(value) {
  if (value === null || value === undefined) return "";
  return String(value).trim();
}

export function normalizePowerLevels(values, fallback = DEFAULT_POWER_LEVELS) {
  const supplied = Array.isArray(values)
    ? values
      .map((level) => Math.floor(Number(level)))
      .filter((level) => Number.isFinite(level) && level > 0)
    : [];
  if (supplied.length) return [...new Set(supplied)].sort((left, right) => left - right);
  return [...fallback];
}

/**
 * Pure transition for the client-side fire gate.
 *
 * inputToken identifies one physical/user input. A small bounded history keeps
 * repeated delivery of the same input from creating another shot.
 */
export function evaluateFireAttempt(
  gate = {},
  attempt = {},
  minimumIntervalMs = DEFAULT_FIRE_INTERVAL_MS
) {
  const now = finiteTimestamp(attempt.now, 0);
  const lastAcceptedAt = finiteTimestamp(gate.lastAcceptedAt);
  const recentInputTokens = Array.isArray(gate.recentInputTokens)
    ? gate.recentInputTokens.map(normalizeInputToken).filter(Boolean).slice(-31)
    : [];
  const inputToken = normalizeInputToken(attempt.inputToken);
  const nextGate = {
    lastAcceptedAt,
    recentInputTokens: inputToken
      ? [...recentInputTokens.filter((token) => token !== inputToken), inputToken]
      : recentInputTokens
  };

  if (inputToken && recentInputTokens.includes(inputToken)) {
    return { accepted: false, reason: "duplicate-input", gate: nextGate };
  }

  const interval = normalizeInterval(minimumIntervalMs);
  if (lastAcceptedAt !== null && now - lastAcceptedAt < interval) {
    return { accepted: false, reason: "throttled", gate: nextGate };
  }

  nextGate.lastAcceptedAt = now;
  return { accepted: true, reason: "accepted", gate: nextGate };
}

export function fireFailureMessage(response) {
  if (!response || response.ok !== false) return "";
  const error = response.error;
  const code = String(error?.code || "").trim().toUpperCase();
  if (SILENT_FIRE_FAILURE_CODES.has(code)) return "";
  if (typeof error?.message === "string" && error.message.trim()) return error.message.trim();
  if (typeof error === "string" && error.trim()) return error.trim();
  return "开炮失败";
}

export function registerShotResolution(seen, event = {}, now = 0, maximumEntries = 128) {
  const shotId = String(event.shotId || event.commandId || "").trim();
  if (!shotId) return true;
  if (seen.has(shotId)) return false;
  seen.set(shotId, finiteTimestamp(now, 0));
  const limit = Math.max(1, Math.floor(finiteTimestamp(maximumEntries, 128)));
  while (seen.size > limit) seen.delete(seen.keys().next().value);
  return true;
}

export function shouldShowFireFailure(response) {
  return fireFailureMessage(response) !== "";
}
