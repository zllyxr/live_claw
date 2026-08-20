import { NOT_LOGIN_TOKEN, NOT_LOGIN_UID, STORAGE_KEYS } from "@/constants/config";
import type { SessionState, UserProfile } from "@/types/api";

let memorySession: SessionState = {
  uid: NOT_LOGIN_UID,
  token: NOT_LOGIN_TOKEN
};
const sessionListeners = new Set<(session: SessionState) => void>();
const beforeSessionClearListeners = new Set<(session: SessionState) => void>();

function notifySessionChange() {
  sessionListeners.forEach((listener) => listener(memorySession));
}

function readString(key: string, fallback = "") {
  try {
    const value = uni.getStorageSync(key);
    return value == null || value === "" ? fallback : String(value);
  } catch {
    return fallback;
  }
}

function writeStorage(key: string, value: unknown) {
  try {
    uni.setStorageSync(key, value);
  } catch {
    // Storage can fail in restricted runtimes; keep memory state usable.
  }
}

export function preloadSession() {
  const uid = readString(STORAGE_KEYS.uid, NOT_LOGIN_UID);
  const token = readString(STORAGE_KEYS.token, NOT_LOGIN_TOKEN);
  const user = uni.getStorageSync(STORAGE_KEYS.user) as UserProfile | undefined;
  memorySession = {
    uid: uid || NOT_LOGIN_UID,
    token: token || NOT_LOGIN_TOKEN,
    user
  };
  return memorySession;
}

export function getSession() {
  if (!memorySession.uid || !memorySession.token) {
    return preloadSession();
  }
  return memorySession;
}

export function isLoggedIn() {
  const session = getSession();
  return Boolean(
    session.uid &&
      session.token &&
      session.uid !== NOT_LOGIN_UID &&
      session.token !== NOT_LOGIN_TOKEN
  );
}

export function saveSession(info: UserProfile) {
  const uid = String(info.id || info.uid || "");
  const token = String(info.token || "");
  if (memorySession.uid !== NOT_LOGIN_UID && memorySession.uid !== uid) {
    beforeSessionClearListeners.forEach((listener) => listener(memorySession));
  }
  memorySession = {
    uid: uid || NOT_LOGIN_UID,
    token: token || NOT_LOGIN_TOKEN,
    user: info
  };
  writeStorage(STORAGE_KEYS.uid, memorySession.uid);
  writeStorage(STORAGE_KEYS.token, memorySession.token);
  writeStorage(STORAGE_KEYS.user, info);
  notifySessionChange();
}

export function saveUser(user: UserProfile) {
  memorySession = {
    ...getSession(),
    user
  };
  writeStorage(STORAGE_KEYS.user, user);
}

export function clearSession() {
  if (memorySession.uid !== NOT_LOGIN_UID && memorySession.token !== NOT_LOGIN_TOKEN) {
    beforeSessionClearListeners.forEach((listener) => listener(memorySession));
  }
  memorySession = {
    uid: NOT_LOGIN_UID,
    token: NOT_LOGIN_TOKEN
  };
  try {
    uni.removeStorageSync(STORAGE_KEYS.uid);
    uni.removeStorageSync(STORAGE_KEYS.token);
    uni.removeStorageSync(STORAGE_KEYS.user);
  } catch {
    // Ignore storage cleanup failures.
  }
  notifySessionChange();
}

export function onSessionChange(listener: (session: SessionState) => void) {
  sessionListeners.add(listener);
  return () => sessionListeners.delete(listener);
}

export function onBeforeSessionClear(listener: (session: SessionState) => void) {
  beforeSessionClearListeners.add(listener);
  return () => beforeSessionClearListeners.delete(listener);
}

export function authParams() {
  const session = getSession();
  return {
    uid: session.uid,
    token: session.token
  };
}

export function requireLogin() {
  if (isLoggedIn()) {
    return true;
  }
  uni.navigateTo({ url: "/pages/auth/login" });
  return false;
}
