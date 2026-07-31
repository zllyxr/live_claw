import { API_HOST, NOT_LOGIN_UID } from "@/constants/config";
import type { RechargeOrder } from "@/types/api";
import { openWebView } from "@/utils/navigation";

export const PENDING_PAYMENT_PREFIX = "payment_pending:";
export const PAYMENT_CREATE_ATTEMPT_PREFIX = "payment_create_attempt:";

export interface PendingPayment {
  uid: string;
  orderNo: string;
  paymentUrl: string;
  providerTradeId: string;
  status: string;
  expiresAt: string;
  channel: string;
  bankStage: string;
  createdAt: number;
}

export interface PaymentCreateAttempt {
  uid: string;
  productId: string;
  payId: string;
  traceId: string;
  createdAt: number;
}

function valueText(value: unknown) {
  if (value === undefined || value === null) {
    return "";
  }
  return String(value).trim();
}

function firstValue(source: RechargeOrder, keys: Array<keyof RechargeOrder>) {
  for (const key of keys) {
    const value = valueText(source[key]);
    if (value) {
      return value;
    }
  }
  return "";
}

export function rechargeOrderNo(order?: RechargeOrder) {
  return order ? firstValue(order, ["order_no", "orderid"]) : "";
}

export function rechargePaymentUrl(order?: RechargeOrder) {
  return order
    ? firstValue(order, ["payment_url", "payurl", "url", "href", "qrcode"])
    : "";
}

export function rechargeProviderTradeId(order?: RechargeOrder) {
  return order ? firstValue(order, ["provider_trade_id", "provider_order_no"]) : "";
}

export function rechargeStatusCode(order?: RechargeOrder) {
  const parsed = Number(valueText(order?.status));
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : 0;
}

export function rechargeStatusText(order?: RechargeOrder) {
  const provided = valueText(order?.status_text);
  if (provided) {
    return provided;
  }
  return ({
    0: "订单已创建",
    1: "等待支付",
    2: "已到账",
    3: "支付失败",
    4: "已关闭",
    5: "已退款"
  } as Record<number, string>)[rechargeStatusCode(order)] || "未知状态";
}

export function rechargeExpiresAt(order?: RechargeOrder) {
  return order ? valueText(order.expires_at) : "";
}

export function expirationTimestamp(value: unknown) {
  const text = valueText(value);
  if (!text) {
    return 0;
  }
  const numeric = Number(text);
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric > 10_000_000_000 ? Math.floor(numeric / 1000) : Math.floor(numeric);
  }
  const parsed = Date.parse(text);
  return Number.isFinite(parsed) ? Math.floor(parsed / 1000) : 0;
}

export function rechargeIsExpired(order?: RechargeOrder, now = Date.now()) {
  if (valueText(order?.bank_stage) === "review_pending") {
    return false;
  }
  const expiresAt = expirationTimestamp(rechargeExpiresAt(order));
  return expiresAt > 0 && expiresAt <= Math.floor(now / 1000);
}

export function rechargeIsPaid(order?: RechargeOrder) {
  return rechargeStatusCode(order) === 2;
}

export function rechargeIsTerminal(order?: RechargeOrder) {
  return [2, 3, 4, 5].includes(rechargeStatusCode(order));
}

export function canContinueRecharge(order?: RechargeOrder, now = Date.now()) {
  if (
    valueText(order?.channel).toLowerCase() === "bank" ||
    valueText(order?.payment_method).toLowerCase() === "bank_transfer"
  ) {
    return (
      [0, 1].includes(rechargeStatusCode(order)) &&
      !rechargeIsExpired(order, now) &&
      ["waiting_assignment", "awaiting_payment", "review_pending"].includes(
        valueText(order?.bank_stage)
      )
    );
  }
  return (
    [0, 1].includes(rechargeStatusCode(order)) &&
    Boolean(normalizePaymentUrl(rechargePaymentUrl(order))) &&
    !rechargeIsExpired(order, now)
  );
}

export function normalizePaymentUrl(value: unknown) {
  const raw = valueText(value);
  if (!raw) {
    return "";
  }
  try {
    const trustedOrigin = new URL(API_HOST);
    const target = raw.startsWith("//")
      ? new URL(`https:${raw}`)
      : raw.startsWith("/")
        ? new URL(raw, `${API_HOST}/`)
        : new URL(raw);
    const localHTTP =
      trustedOrigin.protocol === "http:" &&
      (trustedOrigin.hostname === "localhost" ||
        trustedOrigin.hostname === "127.0.0.1" ||
        trustedOrigin.hostname === "0.0.0.0");
    if (
      target.origin !== trustedOrigin.origin ||
      target.username ||
      target.password ||
      !target.pathname.startsWith("/pay/") ||
      (target.protocol !== "https:" && !localHTTP)
    ) {
      return "";
    }
    return target.href;
  } catch {
    return "";
  }
}

export function openPaymentCashier(value: unknown, title = "支付") {
  const normalized = normalizePaymentUrl(value);
  if (!normalized) {
    return false;
  }

  // #ifdef H5
  const browserLocation = (
    globalThis as unknown as { location?: { assign?: (target: string) => void } }
  ).location;
  if (browserLocation?.assign) {
    browserLocation.assign(normalized);
    return true;
  }
  // #endif

  openWebView(normalized, title);
  return true;
}

export function pendingPaymentKey(uid: string) {
  return `${PENDING_PAYMENT_PREFIX}${uid}`;
}

function validUID(uid: string) {
  const value = valueText(uid);
  return value && value !== NOT_LOGIN_UID ? value : "";
}

export function readPendingPayment(uid: string) {
  const normalizedUID = validUID(uid);
  if (!normalizedUID) {
    return undefined;
  }
  const key = pendingPaymentKey(normalizedUID);
  try {
    const stored = uni.getStorageSync(key) as PendingPayment | string | undefined;
    const parsed =
      typeof stored === "string" && stored.trim()
        ? (JSON.parse(stored) as PendingPayment)
        : stored;
    if (
      !parsed ||
      typeof parsed !== "object" ||
      valueText(parsed.uid) !== normalizedUID ||
      !valueText(parsed.orderNo)
    ) {
      uni.removeStorageSync(key);
      return undefined;
    }
    return {
      uid: normalizedUID,
      orderNo: valueText(parsed.orderNo),
      paymentUrl: normalizePaymentUrl(parsed.paymentUrl),
      providerTradeId: valueText(parsed.providerTradeId),
      status: valueText(parsed.status),
      expiresAt: valueText(parsed.expiresAt),
      channel: valueText(parsed.channel),
      bankStage: valueText(parsed.bankStage),
      createdAt: Number(parsed.createdAt || 0)
    } satisfies PendingPayment;
  } catch {
    try {
      uni.removeStorageSync(key);
    } catch {
      // Storage cleanup is best effort.
    }
    return undefined;
  }
}

export function savePendingPayment(uid: string, order: RechargeOrder) {
  const normalizedUID = validUID(uid);
  const orderNo = rechargeOrderNo(order);
  if (!normalizedUID || !orderNo) {
    return undefined;
  }
  const item: PendingPayment = {
    uid: normalizedUID,
    orderNo,
    paymentUrl: normalizePaymentUrl(rechargePaymentUrl(order)),
    providerTradeId: rechargeProviderTradeId(order),
    status: valueText(order.status),
    expiresAt: rechargeExpiresAt(order),
    channel: valueText(order.channel),
    bankStage: valueText(order.bank_stage),
    createdAt: Date.now()
  };
  try {
    uni.setStorageSync(pendingPaymentKey(normalizedUID), item);
  } catch {
    // The server remains the source of truth when local storage is unavailable.
  }
  return item;
}

export function clearPendingPayment(uid: string) {
  const normalizedUID = validUID(uid);
  if (!normalizedUID) {
    return;
  }
  try {
    uni.removeStorageSync(pendingPaymentKey(normalizedUID));
  } catch {
    // Storage cleanup is best effort.
  }
}

export function clearForeignPendingPayments(currentUID: string) {
  const keepKey = validUID(currentUID) ? pendingPaymentKey(currentUID) : "";
  try {
    const info = uni.getStorageInfoSync() as { keys?: Array<string | number> };
    for (const rawKey of info.keys || []) {
      const key = String(rawKey);
      if (key.startsWith(PENDING_PAYMENT_PREFIX) && key !== keepKey) {
        uni.removeStorageSync(key);
      }
    }
  } catch {
    // Some restricted runtimes do not allow enumerating storage.
  }
}

function paymentCreateAttemptKey(uid: string) {
  return `${PAYMENT_CREATE_ATTEMPT_PREFIX}${uid}`;
}

export function readPaymentCreateAttempt(uid: string) {
  const normalizedUID = validUID(uid);
  if (!normalizedUID) {
    return undefined;
  }
  try {
    const stored = uni.getStorageSync(paymentCreateAttemptKey(normalizedUID)) as
      | PaymentCreateAttempt
      | string
      | undefined;
    const parsed: PaymentCreateAttempt | undefined =
      typeof stored === "string"
        ? stored.trim()
          ? (JSON.parse(stored) as PaymentCreateAttempt)
          : undefined
        : stored;
    const createdAt = Number(parsed?.createdAt || 0);
    if (
      !parsed ||
      valueText(parsed.uid) !== normalizedUID ||
      !valueText(parsed.productId) ||
      !valueText(parsed.payId) ||
      !/^[0-9A-Za-z_-]{8,100}$/.test(valueText(parsed.traceId)) ||
      createdAt < Date.now() - 2 * 60 * 60 * 1000 ||
      createdAt > Date.now() + 60_000
    ) {
      uni.removeStorageSync(paymentCreateAttemptKey(normalizedUID));
      return undefined;
    }
    return {
      uid: normalizedUID,
      productId: valueText(parsed.productId),
      payId: valueText(parsed.payId),
      traceId: valueText(parsed.traceId),
      createdAt
    } satisfies PaymentCreateAttempt;
  } catch {
    try {
      uni.removeStorageSync(paymentCreateAttemptKey(normalizedUID));
    } catch {
      // Storage cleanup is best effort.
    }
    return undefined;
  }
}

export function savePaymentCreateAttempt(attempt: PaymentCreateAttempt) {
  const uid = validUID(attempt.uid);
  if (!uid) {
    return;
  }
  try {
    uni.setStorageSync(paymentCreateAttemptKey(uid), {
      uid,
      productId: valueText(attempt.productId),
      payId: valueText(attempt.payId),
      traceId: valueText(attempt.traceId),
      createdAt: Number(attempt.createdAt || Date.now())
    } satisfies PaymentCreateAttempt);
  } catch {
    // Server-side idempotency still protects transport retries.
  }
}

export function clearPaymentCreateAttempt(uid: string, traceId = "") {
  const normalizedUID = validUID(uid);
  if (!normalizedUID) {
    return;
  }
  if (traceId) {
    const stored = readPaymentCreateAttempt(normalizedUID);
    if (stored && stored.traceId !== traceId) {
      return;
    }
  }
  try {
    uni.removeStorageSync(paymentCreateAttemptKey(normalizedUID));
  } catch {
    // Storage cleanup is best effort.
  }
}
