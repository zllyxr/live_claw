export function displayCount(value: unknown) {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) {
    return "0";
  }
  if (num >= 10000) {
    return `${(num / 10000).toFixed(num >= 100000 ? 0 : 1)}万`;
  }
  return String(num);
}

export function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
