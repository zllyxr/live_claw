import { API_HOST } from "@/constants/config";

function appBaseUrl() {
  // App-plus runs the service bundle in JavaScriptCore, where the browser URL
  // constructor is not guaranteed to exist. Reading import.meta here makes
  // Vite emit `new URL(..., document.baseURI)` into that bundle, so derive the
  // same base from the runtime location instead.
  const base = appBase();
  return base.endsWith("/") ? base : `${base}/`;
}

function slash() {
  return String.fromCharCode(47);
}

function staticPrefix() {
  const s = slash();
  return `${s}static${s}`;
}

function h5StaticPrefix() {
  const s = slash();
  return `${s}h5${s}static${s}`;
}

function isExternalUrl(value: string) {
  return /^https?:\/\//i.test(value) || value.startsWith("//") || value.startsWith("blob:") || value.startsWith("data:");
}

export function absolutizeUrl(value?: string | null) {
  if (!value) {
    return "";
  }
  const raw = String(value).trim();
  if (!raw) {
    return "";
  }
  if (isExternalUrl(raw)) {
    if (raw.startsWith("//")) {
      return `https:${raw}`;
    }
    return raw;
  }
  if (raw.startsWith("/")) {
    return `${API_HOST}${raw}`;
  }
  if (raw.startsWith("local_")) {
    return `${API_HOST}/upload/${raw.slice("local_".length)}`;
  }
  if (raw.startsWith("minio_")) {
    return `${API_HOST}/upload/${raw.slice("minio_".length)}`;
  }
  return `${API_HOST}/upload/${raw.replace(/^\/+/, "")}`;
}

export function staticAsset(value: string) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  if (isExternalUrl(raw)) {
    return raw.startsWith("//") ? `https:${raw}` : raw;
  }
  const normalized = raw.startsWith("/") ? raw : `/${raw}`;
  if (normalized.startsWith(h5StaticPrefix())) {
    return normalized;
  }
  if (normalized.startsWith(staticPrefix())) {
    return `${appBaseUrl()}${normalized.slice(1)}`;
  }
  return raw;
}

export function displayUrl(value?: string | null, fallback = "") {
  const raw = String(value || "").trim();
  if (!raw) {
    return fallback ? staticAsset(fallback) : "";
  }
  if (raw.startsWith(staticPrefix()) || raw.startsWith(h5StaticPrefix())) {
    return staticAsset(raw);
  }
  return absolutizeUrl(raw) || (fallback ? staticAsset(fallback) : "");
}

export function joinQuery(url: string, query: Record<string, string | number | undefined>) {
  const params = Object.entries(query)
    .filter(([, value]) => value !== undefined && value !== "")
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`)
    .join("&");
  return params ? `${url}${url.includes("?") ? "&" : "?"}${params}` : url;
}

export function firstText(...values: unknown[]) {
  for (const value of values) {
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      return String(value);
    }
  }
  return "";
}

/**
 * 解析「打进包里的本地静态资源」路径。
 *
 * 背景：构建期写在代码里的 /static/xxx 会被 vite 插件重写成 /h5/static/xxx，
 * 但**接口在运行时返回**的路径不会经过重写，直接用会打到站点根导致 404。
 * 这里用应用基路径（H5 为 /h5/，App 为 /）手动拼接。
 */
/**
 * 应用基路径。
 * 不依赖构建期注入的 BASE_URL（uni-app H5 下并不可靠），
 * 而是运行时从当前页面路径推导：/h5/index.html → /h5/
 */
function appBase(): string {
  const loc = (globalThis as unknown as { location?: { pathname?: string } }).location;
  const pathname = String(loc?.pathname || "");
  if (!pathname) {
    return "/";
  }
  const dir = pathname.replace(/[^/]*$/, "");
  return dir || "/";
}

export function localAssetUrl(path: string): string {
  const raw = String(path || "").trim();
  if (!raw) {
    return "";
  }
  if (isExternalUrl(raw) || raw.startsWith("data:")) {
    return raw;
  }
  const base = appBase();
  const clean = raw.replace(/^\/+/, "");
  // 若已带基路径（例如构建期已被重写过的字面量），不要重复拼
  if (base !== "/" && ("/" + clean).startsWith(base)) {
    return "/" + clean;
  }
  return base + clean;
}
