import { t } from "@/i18n";

export function displayCount(value: unknown) {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) {
    return "0";
  }
  if (num >= 10000) {
    return t("core.tenThousand", { value: (num / 10000).toFixed(num >= 100000 ? 0 : 1) });
  }
  return String(num);
}

export function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
