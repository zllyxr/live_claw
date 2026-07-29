export const APP_NAME = "星域";
const env = ((import.meta as unknown as { env?: Record<string, unknown> }).env || {}) as Record<string, unknown>;
const locationLike = (globalThis as unknown as { location?: { hostname?: string; origin?: string } }).location;
const previewHost = locationLike?.hostname || "";
const browserOrigin = locationLike?.origin || "";
const isLocalHost =
  previewHost === "localhost" ||
  previewHost === "127.0.0.1" ||
  previewHost === "0.0.0.0" ||
  previewHost.startsWith("192.168.") ||
  previewHost.startsWith("10.") ||
  /^172\.(1[6-9]|2\d|3[0-1])\./.test(previewHost);
export const IS_LOCAL_PREVIEW = Boolean(
  isLocalHost || env.VITE_USE_LOCAL_API === "true"
);
const configuredApiHost = String(env.VITE_API_HOST || "https://tmpai2.com").replace(/\/$/, "");
export const API_HOST = browserOrigin || configuredApiHost;
export const PUBLIC_WEB_URL = String(
  env.VITE_PUBLIC_WEB_URL || (browserOrigin ? `${browserOrigin}/` : `${configuredApiHost}/h5/`)
).replace(/\/?$/, "/");
export const API_BASE = browserOrigin ? "/appapi/" : `${API_HOST}/appapi/`;
// All app compatibility services are exposed by the Go API gateway. Keeping a
// single base URL prevents lottery, sports and mini-game calls from silently
// falling back to the retired legacy backend.
export const CORE_API_BASE = API_BASE;
export const GAME_API_BASE = browserOrigin ? "/gameapi/" : `${API_HOST}/gameapi/`;
export const DEFAULT_LANGUAGE = "zh-cn";
export const NOT_LOGIN_UID = "-9999";
export const NOT_LOGIN_TOKEN = "-9999";
export const SIGN_SALT = "76576076c1f5f657b634e966c8836a06";

export const ACTIVE_TYPES = {
  text: 0,
  image: 1,
  video: 2,
  voice: 3
} as const;

export const STORAGE_KEYS = {
  uid: "claw_uid",
  token: "claw_token",
  user: "claw_user",
  config: "claw_config"
} as const;
