import { API_BASE, CORE_API_BASE, DEFAULT_LANGUAGE, GAME_API_BASE } from "@/constants/config";
import { clearSession, getSession } from "@/utils/session";
import type { ApiEnvelope } from "@/types/api";

type RequestMethod = "GET" | "POST";

export class ApiError<T = unknown> extends Error {
  code: number;
  info: T[];

  constructor(code: number, msg: string, info: T[] = []) {
    super(msg || "请求失败");
    this.name = "ApiError";
    this.code = code;
    this.info = info;
  }
}

function parseJsonPayload(payload: unknown): Record<string, unknown> {
  if (typeof payload === "string") {
    const start = payload.indexOf("{");
    const text = start >= 0 ? payload.slice(start) : payload;
    return JSON.parse(text);
  }
  return (payload || {}) as Record<string, unknown>;
}

function parseEnvelope<T>(payload: unknown): ApiEnvelope<T> {
  const root = parseJsonPayload(payload);
  const dataRaw = root.data;
  const data =
    typeof dataRaw === "string"
      ? parseJsonPayload(dataRaw)
      : ((dataRaw || root) as Record<string, unknown>);
  return {
    code: Number(data.code ?? 0),
    msg: String(data.msg ?? root.msg ?? ""),
    info: Array.isArray(data.info) ? (data.info as T[]) : []
  };
}

function handleLoginInvalid(msg: string, requestUID: unknown, requestToken: unknown) {
  const current = getSession();
  if (
    String(current.uid ?? "") !== String(requestUID ?? "") ||
    String(current.token ?? "") !== String(requestToken ?? "")
  ) {
    return;
  }
  clearSession();
  uni.showToast({ title: msg || "登录已失效", icon: "none" });
  setTimeout(() => {
    uni.navigateTo({ url: `/pages/auth/invalid?msg=${encodeURIComponent(msg || "登录已失效")}` });
  }, 250);
}

export interface ApiRequestOptions {
  method?: RequestMethod;
  auth?: boolean;
  throwOnError?: boolean;
  /** 单次请求超时（毫秒），默认 12000 */
  timeout?: number;
  /** 网络/超时类失败的重试次数，默认 2（业务错误码不重试） */
  retry?: number;
}

const CORE_SERVICE_PREFIXES = ["LotteryGame.", "Sports.", "SportsBet.", "MiniGame.", "App."];

function serviceEndpoint(service: string) {
  if (service.startsWith("MiniGame.")) {
    return GAME_API_BASE;
  }
  return CORE_SERVICE_PREFIXES.some((prefix) => service.startsWith(prefix)) ? CORE_API_BASE : API_BASE;
}

const DEFAULT_TIMEOUT = 12000;
const DEFAULT_RETRY = 2;

/** 判断错误是否值得重试：仅网络层/超时，业务错误码不重试 */
function isRetryable(error: unknown): boolean {
  if (!(error instanceof ApiError)) {
    return false;
  }
  return error.code === -1;
}

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function requestOnce<T = Record<string, unknown>>(
  service: string,
  params: Record<string, unknown> = {},
  options: ApiRequestOptions = {}
): Promise<ApiEnvelope<T>> {
  const session = getSession();
  const data: Record<string, unknown> = {
    service,
    language: DEFAULT_LANGUAGE,
    ...params
  };
  if (options.auth !== false) {
    data.uid = data.uid ?? session.uid;
    data.token = data.token ?? session.token;
  }
  return new Promise((resolve, reject) => {
    uni.request({
      url: serviceEndpoint(service),
      method: options.method || "POST",
      header: { "Content-Type": "application/x-www-form-urlencoded" },
      timeout: options.timeout ?? DEFAULT_TIMEOUT,
      data,
      success: (response) => {
        try {
          const envelope = parseEnvelope<T>(response.data);
          if (envelope.code === 700) {
            handleLoginInvalid(envelope.msg, data.uid, data.token);
          }
          if (options.throwOnError !== false && envelope.code !== 0) {
            reject(new ApiError<T>(envelope.code, envelope.msg, envelope.info));
            return;
          }
          resolve(envelope);
        } catch (error) {
          reject(error);
        }
      },
      fail: (error) => {
        reject(new ApiError(-1, error.errMsg || "网络请求失败"));
      }
    });
  });
}

/**
 * 发起接口请求。网络层失败（含超时）会按指数退避自动重试，
 * 业务错误码（如参数错误、余额不足）不重试，直接抛出。
 */
export async function request<T = Record<string, unknown>>(
  service: string,
  params: Record<string, unknown> = {},
  options: ApiRequestOptions = {}
): Promise<ApiEnvelope<T>> {
  const maxRetry = Math.max(0, options.retry ?? DEFAULT_RETRY);
  let stableParams = params;
  if (options.auth !== false) {
    const session = getSession();
    stableParams = {
      ...params,
      uid: params.uid ?? session.uid,
      token: params.token ?? session.token
    };
  }
  let lastError: unknown;
  for (let attempt = 0; attempt <= maxRetry; attempt++) {
    try {
      return await requestOnce<T>(service, stableParams, options);
    } catch (error) {
      lastError = error;
      if (attempt >= maxRetry || !isRetryable(error)) {
        break;
      }
      // 300ms / 900ms 退避
      await delay(300 * Math.pow(3, attempt));
    }
  }
  throw lastError;
}

export async function firstInfo<T>(
  service: string,
  params: Record<string, unknown> = {},
  options: ApiRequestOptions = {}
) {
  const res = await request<T>(service, params, options);
  return res.info[0] as T | undefined;
}

export async function infoList<T>(
  service: string,
  params: Record<string, unknown> = {},
  options: ApiRequestOptions = {}
) {
  const res = await request<T>(service, params, options);
  return res.info;
}
