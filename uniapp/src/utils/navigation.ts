import { API_HOST } from "@/constants/config";

export type GameZoneIntent = "fishing" | "lottery";

const GAME_ZONE_INTENT_KEY = "claw_game_zone_intent";

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

export function openDetailPage(type: string, title?: string) {
  uni.navigateTo({
    url: `/pages/detail/index?type=${encodeURIComponent(type)}&title=${encodeURIComponent(title || "详情")}`
  });
}

export function openSportsDetail(match: Record<string, unknown>) {
  const matchId =
    match.match_id || match.public_match_id || match.id || match.source_match_id || "";
  if (!matchId) {
    uni.showToast({ title: "赛事编号无效", icon: "none" });
    return;
  }
  uni.navigateTo({
    url: `/pages/sports/detail?match_id=${encodeURIComponent(String(matchId))}`
  });
}
