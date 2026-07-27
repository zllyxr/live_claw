export interface PendingInviteParams {
  code?: string;
  ref?: string;
  clickId?: string;
  savedAt?: number;
  source?: string;
}

const PENDING_INVITE_KEY = "claw_pending_invite";

function cleanText(value: unknown) {
  return value === undefined || value === null ? "" : String(value).trim();
}

export function normalizeInviteCode(value: unknown) {
  return cleanText(value).toUpperCase().replace(/[^A-Z0-9]/g, "");
}

function normalizeClickId(value: unknown) {
  return cleanText(value).replace(/[^A-Za-z0-9_-]/g, "");
}

function safeDecode(value: string) {
  try {
    return decodeURIComponent(value.replace(/\+/g, "%20"));
  } catch {
    return value;
  }
}

export function h5HashQuery() {
  const locationLike = (globalThis as unknown as { location?: { hash?: string } }).location;
  const hash = locationLike?.hash || "";
  const queryText = hash.includes("?") ? hash.slice(hash.indexOf("?") + 1) : "";
  const params: Record<string, string> = {};
  queryText.split("&").forEach((pair) => {
    const [key, value = ""] = pair.split("=");
    if (key) {
      params[safeDecode(key)] = safeDecode(value);
    }
  });
  return params;
}

export function pickInviteParams(query?: Record<string, unknown>) {
  const source = { ...h5HashQuery(), ...(query || {}) };
  const code = normalizeInviteCode(source.code || source.ref || source.invite_code || source.agent_code);
  const ref = normalizeInviteCode(source.ref || source.code || source.invite_code || source.agent_code);
  const clickId = normalizeClickId(source.click_id || source.clickId || source.openinstall_click_id);
  if (!code && !ref && !clickId) {
    return undefined;
  }
  return {
    code,
    ref,
    clickId,
    savedAt: Date.now(),
    source: cleanText(source.source || source.from || "")
  } as PendingInviteParams;
}

export function savePendingInvite(params?: PendingInviteParams) {
  if (!params || (!params.code && !params.ref && !params.clickId)) {
    return;
  }
  try {
    uni.setStorageSync(PENDING_INVITE_KEY, params);
  } catch {
    // Ignore storage failures. The page-level bind form still works.
  }
}

export function getPendingInvite() {
  try {
    const value = uni.getStorageSync(PENDING_INVITE_KEY) as PendingInviteParams | string | undefined;
    if (!value) {
      return undefined;
    }
    if (typeof value === "string") {
      return JSON.parse(value) as PendingInviteParams;
    }
    return value;
  } catch {
    return undefined;
  }
}

export function clearPendingInvite() {
  try {
    uni.removeStorageSync(PENDING_INVITE_KEY);
  } catch {
    // Ignore storage cleanup failures.
  }
}

export function truthyFlag(value: unknown) {
  return Number(value || 0) === 1 || value === true || value === "true";
}
