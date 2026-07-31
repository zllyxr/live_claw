const DEFAULT_WORLD = Object.freeze({ width: 1280, height: 720 });
const SEAT_COUNT = 4;

function finiteNumber(value, fallback = 0) {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}

function worldSize(world = DEFAULT_WORLD) {
  const width = finiteNumber(world?.width, DEFAULT_WORLD.width);
  const height = finiteNumber(world?.height, DEFAULT_WORLD.height);
  return {
    width: width > 0 ? width : DEFAULT_WORLD.width,
    height: height > 0 ? height : DEFAULT_WORLD.height
  };
}

export function normalizeSeat(seat) {
  if (seat === null || seat === undefined || String(seat).trim() === "") {
    throw new TypeError("seat must be a finite zero-based seat number");
  }
  const supplied = Number(seat);
  if (!Number.isInteger(supplied)) {
    throw new TypeError("seat must be a finite zero-based integer");
  }
  return ((supplied % SEAT_COUNT) + SEAT_COUNT) % SEAT_COUNT;
}

export function shouldFlipForViewer(localSeat) {
  return normalizeSeat(localSeat) >= 2;
}

export function serverSeatToDisplaySeat(serverSeat, localSeat) {
  const normalized = normalizeSeat(serverSeat);
  return shouldFlipForViewer(localSeat) ? SEAT_COUNT - 1 - normalized : normalized;
}

export function displaySeatToServerSeat(displaySeat, localSeat) {
  const normalized = normalizeSeat(displaySeat);
  return shouldFlipForViewer(localSeat) ? SEAT_COUNT - 1 - normalized : normalized;
}

export function worldToViewPoint(point, localSeat, world = DEFAULT_WORLD) {
  const size = worldSize(world);
  const x = finiteNumber(point?.x);
  const y = finiteNumber(point?.y);
  if (!shouldFlipForViewer(localSeat)) return { x, y };
  return { x: size.width - x, y: size.height - y };
}

export function viewToWorldPoint(point, localSeat, world = DEFAULT_WORLD) {
  return worldToViewPoint(point, localSeat, world);
}

export function normalizeAngle(angle) {
  const numeric = finiteNumber(angle);
  return Math.atan2(Math.sin(numeric), Math.cos(numeric));
}

export function worldToViewAngle(angle, localSeat) {
  return normalizeAngle(finiteNumber(angle) + (shouldFlipForViewer(localSeat) ? Math.PI : 0));
}

export function viewToWorldAngle(angle, localSeat) {
  return worldToViewAngle(angle, localSeat);
}

export { DEFAULT_WORLD };
