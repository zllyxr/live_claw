export interface ResolvedLiveStream {
  src: string;
  page?: string;
  status?: string;
  reason?: string;
}

const PLAYABLE_PATTERN = /^(rtmp:\/\/|https?:\/\/).*(\.m3u8|\.flv|\.mp4)(\?|#|$)/i;

export function deepDecode(value: unknown) {
  let text = String(value || "").trim();
  for (let index = 0; index < 4; index += 1) {
    try {
      const next = decodeURIComponent(text);
      if (next === text) {
        break;
      }
      text = next;
    } catch {
      break;
    }
  }
  return text.replace(/\\\//g, "/");
}

export function isPlayableLiveUrl(value: string) {
  return PLAYABLE_PATTERN.test(value);
}

export async function resolveLiveStream(raw: unknown): Promise<ResolvedLiveStream> {
  const source = deepDecode(raw);
  if (!source) {
    return { src: "", reason: "直播地址暂不可用" };
  }
  if (isPlayableLiveUrl(source)) {
    return { src: source };
  }
  return { src: "", reason: "直播直拉地址尚未就绪" };
}
