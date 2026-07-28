import { API_HOST, DEFAULT_LANGUAGE } from "@/constants/config";
import { getSession } from "@/utils/session";
import { joinQuery } from "@/utils/url";

export type GameZoneIntent = "fishing" | "lottery";

const GAME_ZONE_INTENT_KEY = "claw_game_zone_intent";

function appendAuthFragment(url: string, uid: string, token: string) {
  const params = new URLSearchParams();
  if (uid) {
    params.set("uid", uid);
  }
  if (token) {
    params.set("token", token);
  }
  const fragment = params.toString();
  return fragment ? `${url}#${fragment}` : url;
}

export function openWebView(url: string, title = "星域", orientation = "auto") {
  uni.navigateTo({
    url: `/pages/webview/index?title=${encodeURIComponent(title)}&orientation=${encodeURIComponent(orientation)}&url=${encodeURIComponent(url)}`
  });
}

export function openGameView(url: string) {
  uni.navigateTo({
    url: `/pages/gameview/index?url=${encodeURIComponent(url)}`
  });
}

export function openGameZone(zone: GameZoneIntent) {
  try {
    uni.setStorageSync(GAME_ZONE_INTENT_KEY, zone);
  } catch {
    // The game tab still opens even when storage is unavailable.
  }
  uni.switchTab({ url: "/pages/tabbar/game/index" });
}

export function consumeGameZoneIntent(): GameZoneIntent | "" {
  try {
    const value = String(uni.getStorageSync(GAME_ZONE_INTENT_KEY) || "");
    uni.removeStorageSync(GAME_ZONE_INTENT_KEY);
    return value === "lottery" || value === "fishing" ? value : "";
  } catch {
    return "";
  }
}

export function normalizePageUrl(value?: string | null) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "";
  }
  if (raw.startsWith("//")) {
    return `https:${raw}`;
  }
  if (/^https?:\/\//i.test(raw)) {
    return raw;
  }
  if (raw.startsWith("/")) {
    return `${API_HOST}${raw}`;
  }
  return "";
}

export function buildGameDetailUrl(game: { id?: string | number; game_code?: string }) {
  const session = getSession();
  const url = joinQuery(`${API_HOST}/appapi/lotterygame/detail`, {
    game_id: game.id,
    game_code: game.game_code,
    language: DEFAULT_LANGUAGE
  });
  return appendAuthFragment(url, session.uid, session.token);
}

export function buildGameRecordUrl(game?: { id?: string | number; game_code?: string }) {
  const session = getSession();
  const url = joinQuery(`${API_HOST}/appapi/lotterygame/record`, {
    game_id: game?.id,
    game_code: game?.game_code,
    language: DEFAULT_LANGUAGE
  });
  return appendAuthFragment(url, session.uid, session.token);
}

export function buildAppPageUrl(path: string, query: Record<string, string | number | undefined> = {}) {
  const session = getSession();
  const cleanPath = path.replace(/^\/+/, "");
  return joinQuery(`${API_HOST}/${cleanPath}`, {
    uid: session.uid,
    token: session.token,
    language: DEFAULT_LANGUAGE,
    ...query
  });
}

export function buildDetailUrl() {
  return buildAppPageUrl("appapi/Detail/index");
}

export function buildChargeDetailUrl() {
  return buildAppPageUrl("appapi/charge/index");
}

export function buildCashRecordUrl() {
  return buildAppPageUrl("appapi/cash/index");
}

export function buildAuthUrl() {
  return buildAppPageUrl("appapi/Auth/index");
}

export function buildContributeUrl(liveUid: string | number) {
  return buildAppPageUrl("appapi/contribute/index", {
    uid: liveUid
  });
}

export function openDetailPage(type: string, title?: string) {
  uni.navigateTo({
    url: `/pages/detail/index?type=${encodeURIComponent(type)}&title=${encodeURIComponent(title || "详情")}`
  });
}

export function buildSportsDetailUrl(match: Record<string, unknown>) {
  const session = getSession();
  const matchId =
    match.match_id || match.public_match_id || match.id || match.source_match_id || "";
  const url = joinQuery(`${API_HOST}/appapi/sports/detail`, {
    match_id: String(matchId),
    competition_type: String(match.competition_type || match.league_name || ""),
    language: DEFAULT_LANGUAGE
  });
  return appendAuthFragment(url, session.uid, session.token);
}
