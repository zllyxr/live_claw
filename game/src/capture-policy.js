import { randomInt } from 'node:crypto';

export const DEFAULT_RTP = 0.72;
export const CAPTURE_ROLL_SCALE = 1_000_000;
export const BET_LEVELS = Object.freeze([1, 2, 5, 10, 20, 50]);

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(maximum, value));
}

function finiteNumber(value) {
  const number = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(number) ? number : null;
}

export function normaliseRtp(value = DEFAULT_RTP) {
  return clamp(finiteNumber(value) ?? DEFAULT_RTP, 0, 1);
}

export function normaliseBet(value, fallback = BET_LEVELS[0]) {
  const requested = finiteNumber(value);
  if (requested === null) return BET_LEVELS.includes(fallback) ? fallback : BET_LEVELS[0];

  return BET_LEVELS.reduce((nearest, candidate) => {
    const candidateDistance = Math.abs(candidate - requested);
    const nearestDistance = Math.abs(nearest - requested);
    return candidateDistance < nearestDistance ? candidate : nearest;
  }, BET_LEVELS[0]);
}

export function captureProbability(multiplier, rtp = DEFAULT_RTP) {
  const safeMultiplier = Math.max(1, finiteNumber(multiplier) ?? 1);
  return clamp(normaliseRtp(rtp) / safeMultiplier, 0, 1);
}

// Capture randomness is deliberately independent from the Math.random stream used
// for cosmetic fish spawning and routes. Tests can inject a deterministic function
// that returns a unit value in [0, 1] (1 is accepted as an explicit forced miss).
export function cryptoCaptureRng() {
  return randomInt(0, CAPTURE_ROLL_SCALE) / CAPTURE_ROLL_SCALE;
}

export function resolveCapture({ multiplier, rtp = DEFAULT_RTP, captureRng = cryptoCaptureRng }) {
  if (typeof captureRng !== 'function') throw new TypeError('captureRng must be a function');

  const probability = captureProbability(multiplier, rtp);
  const rawRoll = finiteNumber(captureRng());
  if (rawRoll === null || rawRoll < 0 || rawRoll > 1) {
    throw new RangeError('captureRng must return a finite number in [0, 1]');
  }

  return {
    captured: rawRoll < probability,
    probability,
  };
}
