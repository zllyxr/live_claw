import assert from 'node:assert/strict';
import test from 'node:test';
import { io as createSocket } from 'socket.io-client';
import { createCardGameServer } from '../src/server.js';

function ack(socket, event, payload) {
  return new Promise((resolve, reject) => {
    socket.timeout(3000).emit(event, payload, (error, result) => {
      if (error) reject(error);
      else resolve(result);
    });
  });
}

function nextEvent(socket, event) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`等待 ${event} 超时`)), 3000);
    socket.once(event, (payload) => {
      clearTimeout(timer);
      resolve(payload);
    });
  });
}

function nextMatchingEvent(socket, event, predicate, timeoutMs = 3000) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      socket.off(event, handler);
      reject(new Error(`等待符合条件的 ${event} 超时`));
    }, timeoutMs);
    const handler = (payload) => {
      if (!predicate(payload)) return;
      clearTimeout(timer);
      socket.off(event, handler);
      resolve(payload);
    };
    socket.on(event, handler);
  });
}

test('斗地主：三位平台用户进入同一匹配桌后才开局并统一扣入桌金', async (t) => {
  const balances = new Map([[1, 10_000], [2, 10_000], [3, 10_000]]);
  const orders = [];
  const wallet = {
    async balance(uid) {
      return { ok: true, balance: balances.get(uid) };
    },
    async adjust(order) {
      const balance = balances.get(order.uid) + order.amount;
      if (balance < 0) throw new Error('钱包余额不足');
      balances.set(order.uid, balance);
      orders.push(order);
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({ authRequired: false, wallet });
  const http = await server.listen(0);
  const { port } = http.address();
  const sockets = [1, 2, 3].map(() => createSocket(`http://127.0.0.1:${port}`, {
    transports: ['websocket'],
    forceNew: true
  }));
  t.after(async () => {
    sockets.forEach((socket) => socket.disconnect());
    await server.close();
  });
  await Promise.all(sockets.map((socket) => nextEvent(socket, 'connect')));

  const statePromises = sockets.map((socket) => nextEvent(socket, 'game:state'));
  for (let index = 0; index < sockets.length; index++) {
    const joined = await ack(sockets[index], 'match:join', {
      game: 'ddz',
      uid: index + 1,
      name: `真人${index + 1}`,
      table: 7
    });
    assert.equal(joined.ok, true);
    assert.equal(joined.tableNo, 7);
    assert.equal(joined.tableCount, 1000);
  }
  const states = await Promise.all(statePromises);
  assert.deepEqual(states.map((state) => state.seat), [0, 1, 2]);
  assert.deepEqual(states[0].seatNames, ['真人1', '真人2', '真人3']);
  assert.equal(states[0].myHand.length, 17);
  assert.equal(states[1].myHand.length, 17);
  assert.equal(states[2].myHand.length, 17);
  assert.equal(orders.filter((order) => order.reason === 'round_buy_in').length, 3);
  assert.deepEqual([...balances.values()], [9_900, 9_900, 9_900]);

  const bid1 = await ack(sockets[0], 'game:action', { action: 'bid', call: true });
  assert.equal(bid1.ok, true);
  const bid2 = await ack(sockets[1], 'game:action', { action: 'bid', call: false });
  assert.equal(bid2.ok, true);
  const playing = nextEvent(sockets[0], 'game:state');
  const bid3 = await ack(sockets[2], 'game:action', { action: 'bid', call: false });
  assert.equal(bid3.ok, true);
  assert.equal((await playing).phase, 'playing');
});

test('红中麻将：四位平台用户匹配后进入红中赖子规则牌桌', async (t) => {
  const balances = new Map([[11, 2_000], [12, 2_000], [13, 2_000], [14, 2_000]]);
  const wallet = {
    async balance(uid) {
      return { ok: true, balance: balances.get(uid) };
    },
    async adjust(order) {
      const balance = balances.get(order.uid) + order.amount;
      if (balance < 0) throw new Error('钱包余额不足');
      balances.set(order.uid, balance);
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({ authRequired: false, wallet });
  const http = await server.listen(0);
  const { port } = http.address();
  const sockets = [11, 12, 13, 14].map(() => createSocket(`http://127.0.0.1:${port}`, {
    transports: ['websocket'],
    forceNew: true
  }));
  t.after(async () => {
    sockets.forEach((socket) => socket.disconnect());
    await server.close();
  });
  await Promise.all(sockets.map((socket) => nextEvent(socket, 'connect')));

  const lobby = await fetch(`http://127.0.0.1:${port}/api/lobby/mahjong_red`).then((res) => res.json());
  assert.equal(lobby.ok, true);
  assert.equal(lobby.tableCount, 1000);

  const statePromises = sockets.map((socket) => nextEvent(socket, 'game:state'));
  for (let index = 0; index < sockets.length; index++) {
    const joined = await ack(sockets[index], 'match:join', {
      game: 'mahjong_red',
      uid: index + 11,
      name: `麻友${index + 1}`,
      table: 88
    });
    assert.equal(joined.ok, true);
    assert.equal(joined.requiredPlayers, 4);
  }
  const states = await Promise.all(statePromises);
  assert.deepEqual(states.map((state) => state.seat), [0, 1, 2, 3]);
  assert.ok(states.every((state) => state.ruleset === 'red-center'));
  assert.ok(states.every((state) => state.game === 'mahjong_red'));
  assert.deepEqual([...balances.values()], [1_900, 1_900, 1_900, 1_900]);
});

test('机器人兜底：五种牌桌单个真人都能自动开局，且机器人不访问真实钱包', async (t) => {
  const balances = new Map();
  const orders = [];
  const wallet = {
    async balance(uid) {
      assert.equal(typeof uid, 'number', '机器人不能查询真实钱包');
      if (!balances.has(uid)) balances.set(uid, 10_000);
      return { ok: true, balance: balances.get(uid) };
    },
    async adjust(order) {
      assert.equal(typeof order.uid, 'number', '机器人不能产生真实钱包订单');
      const balance = balances.get(order.uid) + order.amount;
      if (balance < 0) throw new Error('钱包余额不足');
      balances.set(order.uid, balance);
      orders.push(order);
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({
    authRequired: false,
    wallet,
    botFillDelayMs: 5,
    botActionMinMs: 5,
    botActionMaxMs: 15,
    humanTurnTimeoutMs: 500,
    random: () => 0.5
  });
  const http = await server.listen(0);
  const { port } = http.address();
  const sockets = [];
  t.after(async () => {
    sockets.forEach((socket) => socket.disconnect());
    await server.close();
  });

  const cases = [
    { kind: 'ddz', seats: 3 },
    { kind: 'mahjong', seats: 4 },
    { kind: 'mahjong_red', seats: 4 },
    { kind: 'paodekuai', seats: 3 },
    { kind: 'zhajinhua', seats: 3 }
  ];
  for (let index = 0; index < cases.length; index += 1) {
    const { kind, seats } = cases[index];
    const uid = 500 + index;
    const tableNo = 900 + index;
    const socket = createSocket(`http://127.0.0.1:${port}`, {
      transports: ['websocket'],
      forceNew: true
    });
    sockets.push(socket);
    await nextEvent(socket, 'connect');

    const statePromise = nextEvent(socket, 'game:state');
    const joined = await ack(socket, 'match:join', {
      game: kind,
      uid,
      name: `单人玩家${index + 1}`,
      table: tableNo
    });
    assert.equal(joined.ok, true);
    const state = await statePromise;
    assert.equal(state.game, kind);
    assert.equal(state.seat, 0);
    assert.equal(state.seatNames.length, seats);

    const table = server.tables.get(`${kind}:${tableNo}`);
    assert.equal(table.status, 'playing');
    assert.equal(table.players.size, seats);
    assert.equal([...table.players.values()].filter((player) => player.isBot).length, seats - 1);
    assert.equal([...table.players.values()].filter((player) => !player.isBot).length, 1);
  }

  const buyIns = orders.filter((order) => order.reason === 'round_buy_in');
  assert.equal(buyIns.length, cases.length);
  assert.ok(buyIns.every((order) => typeof order.uid === 'number'));
  assert.ok([...balances.values()].every((balance) => balance === 9_900));

  const health = await fetch(`http://127.0.0.1:${port}/health`).then((res) => res.json());
  assert.equal(health.botsEnabled, true);
  assert.equal(health.botActionMinMs, 5);
  assert.equal(health.botActionMaxMs, 15);
  assert.equal(health.humanTurnTimeoutMs, 500);
  assert.equal(health.humanPlayers, cases.length);
  assert.equal(health.botPlayers, cases.reduce((total, item) => total + item.seats - 1, 0));
});

test('真人超时托管：跑得快玩家不操作也会自动出牌，牌桌不会卡死', async (t) => {
  const balances = new Map([[701, 10_000], [702, 10_000], [703, 10_000]]);
  const wallet = {
    async balance(uid) {
      return { ok: true, balance: balances.get(uid) };
    },
    async adjust(order) {
      const balance = balances.get(order.uid) + order.amount;
      balances.set(order.uid, balance);
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({
    authRequired: false,
    botsEnabled: false,
    wallet,
    humanTurnTimeoutMs: 120
  });
  const http = await server.listen(0);
  const { port } = http.address();
  const sockets = [701, 702, 703].map(() => createSocket(`http://127.0.0.1:${port}`, {
    transports: ['websocket'],
    forceNew: true
  }));
  t.after(async () => {
    sockets.forEach((socket) => socket.disconnect());
    await server.close();
  });
  await Promise.all(sockets.map((socket) => nextEvent(socket, 'connect')));

  const initialStates = sockets.map((socket) => nextEvent(socket, 'game:state'));
  for (let index = 0; index < sockets.length; index += 1) {
    const joined = await ack(sockets[index], 'match:join', {
      game: 'paodekuai',
      uid: 701 + index,
      name: `超时测试${index + 1}`,
      table: 777
    });
    assert.equal(joined.ok, true);
  }
  const first = (await Promise.all(initialStates))[0];
  assert.ok(first.turnDeadline > Date.now());
  assert.equal(first.turnSeat, first.current);

  const advanced = await nextMatchingEvent(
    sockets[0],
    'game:state',
    (state) => state.history.some((item) => item.type === 'timeout'),
    1500
  );
  const timedOutSeat = advanced.history.find((item) => item.type === 'timeout').seat;
  assert.ok(advanced.handCounts[timedOutSeat] < 16);
});

test('机器人思考时间：动作经过随机区间调度，不会收到状态后立即秒出', async (t) => {
  let balance = 10_000;
  const wallet = {
    async balance() {
      return { ok: true, balance };
    },
    async adjust(order) {
      balance += order.amount;
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({
    authRequired: false,
    wallet,
    botFillDelayMs: 5,
    botActionMinMs: 80,
    botActionMaxMs: 120,
    humanTurnTimeoutMs: 1000,
    random: () => 0.5
  });
  const http = await server.listen(0);
  const { port } = http.address();
  const socket = createSocket(`http://127.0.0.1:${port}`, {
    transports: ['websocket'],
    forceNew: true
  });
  t.after(async () => {
    socket.disconnect();
    await server.close();
  });
  await nextEvent(socket, 'connect');
  const initialState = nextEvent(socket, 'game:state');
  const joined = await ack(socket, 'match:join', {
    game: 'zhajinhua',
    uid: 801,
    name: '机器人节奏测试',
    table: 778
  });
  assert.equal(joined.ok, true);
  assert.equal((await initialState).current, 0);

  const botScheduled = nextEvent(socket, 'game:state');
  const startedAt = Date.now();
  assert.equal((await ack(socket, 'game:action', { action: 'check' })).ok, true);
  const scheduledState = await botScheduled;
  assert.equal(scheduledState.turnSeat, 1);
  assert.ok(scheduledState.turnDeadline - Date.now() >= 60);

  const advanced = await nextMatchingEvent(
    socket,
    'game:state',
    (state) => state.history.some((item) => item.seat === 1 && item.type === 'check'),
    1000
  );
  assert.equal(advanced.current, 2);
  assert.ok(Date.now() - startedAt >= 70);
});

test('退出不逃单：真人全部退出后牌局继续托管，并按原钱包完成输赢结算', async (t) => {
  const balances = new Map([[901, 10_000], [902, 10_000], [903, 10_000]]);
  const orders = [];
  const wallet = {
    async balance(uid) {
      return { ok: true, balance: balances.get(uid) };
    },
    async adjust(order) {
      const balance = balances.get(order.uid) + order.amount;
      balances.set(order.uid, balance);
      orders.push(order);
      return { ok: true, balance };
    }
  };
  const server = createCardGameServer({
    authRequired: false,
    botsEnabled: false,
    wallet,
    botActionMinMs: 1,
    botActionMaxMs: 1,
    humanTurnTimeoutMs: 1000
  });
  const http = await server.listen(0);
  const { port } = http.address();
  const sockets = [901, 902, 903].map(() => createSocket(`http://127.0.0.1:${port}`, {
    transports: ['websocket'],
    forceNew: true
  }));
  t.after(async () => {
    sockets.forEach((socket) => socket.disconnect());
    await server.close();
  });
  await Promise.all(sockets.map((socket) => nextEvent(socket, 'connect')));

  const initialStates = sockets.map((socket) => nextEvent(socket, 'game:state'));
  for (let index = 0; index < sockets.length; index += 1) {
    const joined = await ack(sockets[index], 'match:join', {
      game: 'paodekuai',
      uid: 901 + index,
      name: `退出测试${index + 1}`,
      table: 779
    });
    assert.equal(joined.ok, true);
  }
  await Promise.all(initialStates);
  const leaves = await Promise.all(sockets.map((socket) => ack(socket, 'match:leave', {})));
  assert.ok(leaves.every((result) => result.mode === 'trustee'));

  const table = server.tables.get('paodekuai:779');
  const deadline = Date.now() + 3000;
  while (!table.settled && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(table.status, 'finished');
  assert.equal(table.settled, true);
  assert.ok([...table.players.values()].every((player) => player.connected === false));
  assert.equal(orders.filter((order) => order.reason === 'round_buy_in').length, 3);
  assert.equal(orders.filter((order) => order.reason === 'round_payout').length, 1);
  assert.deepEqual([...balances.values()].sort((a, b) => a - b), [9_900, 9_900, 10_200]);
});
