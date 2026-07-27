/**
 * 用户等级 → 勋章图映射
 *
 * 素材来自 AI 生成的 6 枚勋章（青铜→皇冠），按等级区间归档。
 * 服务端 level 字段范围不固定，这里做上下夹取，保证任何值都能拿到图。
 */
const MEDALS: readonly string[] = [
  "/static/art/medal/lv1.webp",
  "/static/art/medal/lv2.webp",
  "/static/art/medal/lv3.webp",
  "/static/art/medal/lv4.webp",
  "/static/art/medal/lv5.webp",
  "/static/art/medal/lv6.webp"
];

/** 每档覆盖的等级跨度，例：level 1-5 → lv1，6-10 → lv2 */
const SPAN = 5;

export function medalForLevel(level: unknown): string {
  const value = Number(level) || 0;
  const fallback = MEDALS[0] as string;
  if (value <= 0) {
    return fallback;
  }
  const index = Math.min(MEDALS.length - 1, Math.floor((value - 1) / SPAN));
  return MEDALS[index] ?? fallback;
}

/** 名次奖牌（排行榜前三） */
export function rankMedal(rank: number): string {
  if (rank === 1) return "/static/icons/medal-1.svg";
  if (rank === 2) return "/static/icons/medal-2.svg";
  if (rank === 3) return "/static/icons/medal-3.svg";
  return "";
}
