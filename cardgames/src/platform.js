import { createHmac, timingSafeEqual } from 'node:crypto';

export const TABLE_COUNT = 1000;
export const BUY_IN = 100;

function safeEqual(left, right) {
  const a = Buffer.from(String(left || ''));
  const b = Buffer.from(String(right || ''));
  return a.length === b.length && timingSafeEqual(a, b);
}

export function verifyLaunchTicket(input, gameCode, secret, now = Date.now()) {
  const uid = Number(input?.uid);
  const ts = Number(input?.ts);
  const table = Number(input?.table);
  const sig = String(input?.sig || '');
  if (!Number.isSafeInteger(uid) || uid < 1) throw new Error('登录身份无效，请从平台重新进入');
  if (!Number.isSafeInteger(ts) || Math.abs(Math.floor(now / 1000) - ts) > 30 * 60) {
    throw new Error('进入凭证已过期，请从平台重新匹配');
  }
  if (!Number.isInteger(table) || table < 1 || table > TABLE_COUNT) {
    throw new Error('匹配桌号无效');
  }
  const expected = createHmac('sha256', secret)
    .update(`${gameCode}|${uid}|${ts}`)
    .digest('hex')
    .slice(0, 32);
  if (!secret || !safeEqual(expected, sig)) throw new Error('进入凭证校验失败');
  return {
    uid,
    table,
    name: String(input?.name || `玩家${uid}`).trim().slice(0, 20) || `玩家${uid}`
  };
}

export class PlatformWallet {
  constructor(options = {}) {
    this.baseURL = String(options.baseURL ?? process.env.CORE_INTERNAL_URL ?? '').replace(/\/+$/, '');
    this.secret = String(options.secret ?? process.env.MINIGAME_SECRET ?? '');
    this.fetch = options.fetch ?? globalThis.fetch;
    this.disabled = options.disabled ?? !this.baseURL;
    this.mockBalances = new Map();
  }

  async request(path, payload) {
    if (this.disabled) {
      const before = this.mockBalances.get(payload.uid) ?? 10_000;
      const balance = path.endsWith('/adjust') ? before + Number(payload.amount || 0) : before;
      if (balance < 0) throw new Error('钱包余额不足');
      this.mockBalances.set(payload.uid, balance);
      return { ok: true, balance };
    }
    const response = await this.fetch(`${this.baseURL}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Minigame-Secret': this.secret
      },
      body: JSON.stringify(payload),
      signal: AbortSignal.timeout(5000)
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok || !data.ok) throw new Error(data.message || '平台钱包暂不可用');
    return data;
  }

  balance(uid) {
    return this.request('/v1/minigame/wallet/balance', { uid });
  }

  adjust(payload) {
    return this.request('/v1/minigame/wallet/adjust', payload);
  }
}
