import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = await readFile(
  new URL("../fishingassets/perspective.js", import.meta.url),
  "utf8"
);
const perspective = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`
);

const {
  DEFAULT_WORLD,
  displaySeatToServerSeat,
  normalizeAngle,
  normalizeSeat,
  serverSeatToDisplaySeat,
  shouldFlipForViewer,
  viewToWorldAngle,
  viewToWorldPoint,
  worldToViewAngle,
  worldToViewPoint
} = perspective;

const SEAT_ORIGINS = Object.freeze([
  Object.freeze({ x: 430, y: 690 }),
  Object.freeze({ x: 850, y: 690 }),
  Object.freeze({ x: 430, y: 30 }),
  Object.freeze({ x: 850, y: 30 })
]);

const EPSILON = 1e-9;

function assertNear(actual, expected, message) {
  assert.ok(
    Math.abs(actual - expected) <= EPSILON,
    `${message}: expected ${expected}, received ${actual}`
  );
}

function assertPointNear(actual, expected, message) {
  assertNear(actual.x, expected.x, `${message} x`);
  assertNear(actual.y, expected.y, `${message} y`);
}

function angleDelta(actual, expected) {
  return normalizeAngle(actual - expected);
}

function distance(from, to) {
  return Math.hypot(to.x - from.x, to.y - from.y);
}

test("normalizes all seat values into the four zero-based server seats", () => {
  assert.deepEqual(
    [-5, -1, 0, 1, 2, 3, 4, 7, "3"].map(normalizeSeat),
    [3, 3, 0, 1, 2, 3, 0, 3, 3]
  );
  for (const invalid of [undefined, null, "", " ", 1.5, Number.NaN, Number.POSITIVE_INFINITY]) {
    assert.throws(() => normalizeSeat(invalid), TypeError);
  }
});

test("only top-side viewers receive the 180-degree perspective", () => {
  assert.deepEqual(
    [0, 1, 2, 3].map(shouldFlipForViewer),
    [false, false, true, true]
  );
});

test("the local cannon is on the landscape bottom edge for every server seat", () => {
  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    const displaySeat = serverSeatToDisplaySeat(localSeat, localSeat);
    const displayOrigin = SEAT_ORIGINS[displaySeat];
    assert.ok(
      displayOrigin.y > DEFAULT_WORLD.height / 2,
      `server seat ${localSeat} mapped to display seat ${displaySeat} at y=${displayOrigin.y}`
    );
  }
});

test("seat remapping is a bijection and its inverse restores the server seat", () => {
  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    const displayed = [];
    for (let serverSeat = 0; serverSeat < 4; serverSeat += 1) {
      const displaySeat = serverSeatToDisplaySeat(serverSeat, localSeat);
      displayed.push(displaySeat);
      assertPointNear(
        SEAT_ORIGINS[displaySeat],
        worldToViewPoint(SEAT_ORIGINS[serverSeat], localSeat),
        `viewer ${localSeat}, server seat ${serverSeat} display origin`
      );
      assert.equal(
        displaySeatToServerSeat(displaySeat, localSeat),
        serverSeat,
        `viewer ${localSeat}, server seat ${serverSeat}`
      );
    }
    assert.deepEqual([...displayed].sort(), [0, 1, 2, 3]);
  }
});

test("world points and touch points round-trip for all four viewer seats", () => {
  const points = [
    { x: 0, y: 0 },
    { x: 1280, y: 720 },
    { x: 640, y: 360 },
    { x: 173.25, y: 614.75 }
  ];

  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    for (const worldPoint of points) {
      const viewPoint = worldToViewPoint(worldPoint, localSeat, DEFAULT_WORLD);
      assertPointNear(
        viewToWorldPoint(viewPoint, localSeat, DEFAULT_WORLD),
        worldPoint,
        `viewer ${localSeat} point round trip`
      );
    }
  }
});

test("point transforms preserve world distances and bullet segment lengths", () => {
  const start = { x: 430, y: 690 };
  const end = { x: 901.5, y: 211.25 };
  const worldLength = distance(start, end);

  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    const viewStart = worldToViewPoint(start, localSeat);
    const viewEnd = worldToViewPoint(end, localSeat);
    assertNear(
      distance(viewStart, viewEnd),
      worldLength,
      `viewer ${localSeat} bullet length`
    );
  }
});

test("world and view angles round-trip modulo one full rotation", () => {
  const angles = [-Math.PI, -2.4, -Math.PI / 2, 0, 0.73, Math.PI / 2, Math.PI];

  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    for (const worldAngle of angles) {
      const viewAngle = worldToViewAngle(worldAngle, localSeat);
      const restored = viewToWorldAngle(viewAngle, localSeat);
      assertNear(
        angleDelta(restored, worldAngle),
        0,
        `viewer ${localSeat} angle ${worldAngle}`
      );
    }
  }
});

test("transformed cannon rays still hit the same transformed target", () => {
  const targets = [
    { x: 640, y: 360 },
    { x: 1010, y: 172 },
    { x: 222, y: 498 }
  ];

  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    const worldOrigin = SEAT_ORIGINS[localSeat];
    const viewOrigin = worldToViewPoint(worldOrigin, localSeat);
    const mappedSeatOrigin = SEAT_ORIGINS[serverSeatToDisplaySeat(localSeat, localSeat)];
    assertPointNear(viewOrigin, mappedSeatOrigin, `viewer ${localSeat} cannon origin`);

    for (const worldTarget of targets) {
      const worldAngle = Math.atan2(
        worldTarget.y - worldOrigin.y,
        worldTarget.x - worldOrigin.x
      );
      const viewTarget = worldToViewPoint(worldTarget, localSeat);
      const expectedViewAngle = Math.atan2(
        viewTarget.y - viewOrigin.y,
        viewTarget.x - viewOrigin.x
      );
      assertNear(
        angleDelta(worldToViewAngle(worldAngle, localSeat), expectedViewAngle),
        0,
        `viewer ${localSeat} target ${JSON.stringify(worldTarget)}`
      );
    }
  }
});

test("a view touch produces the same server-world aim and visible trajectory", () => {
  const viewTouches = [
    { x: 640, y: 360 },
    { x: 1040, y: 140 },
    { x: 185, y: 525 }
  ];

  for (let localSeat = 0; localSeat < 4; localSeat += 1) {
    const serverOrigin = SEAT_ORIGINS[localSeat];
    const viewOrigin = worldToViewPoint(serverOrigin, localSeat);
    for (const viewTouch of viewTouches) {
      const serverTarget = viewToWorldPoint(viewTouch, localSeat);
      const serverAngle = Math.atan2(
        serverTarget.y - serverOrigin.y,
        serverTarget.x - serverOrigin.x
      );
      const visibleAngle = Math.atan2(
        viewTouch.y - viewOrigin.y,
        viewTouch.x - viewOrigin.x
      );
      assertNear(
        angleDelta(worldToViewAngle(serverAngle, localSeat), visibleAngle),
        0,
        `viewer ${localSeat} touch ${JSON.stringify(viewTouch)}`
      );
      assertPointNear(
        worldToViewPoint(serverTarget, localSeat),
        viewTouch,
        `viewer ${localSeat} visible touch target`
      );
    }
  }
});
