import { API_HOST } from "@/constants/config";
import { getSession } from "@/utils/session";

export interface SupportConversation {
  id: string;
  subject?: string;
  status?: number;
}

export interface SupportMessage {
  id: string;
  conversation_id: string;
  sender_type: number;
  sender_id: number;
  client_message_id?: string;
  message_type: number;
  text_content?: string;
  asset_id?: number;
  created_at: number;
}

interface NativeEnvelope<T> {
  code: number;
  message?: string;
  data?: T;
}

function supportBase() {
  return `${API_HOST.replace(/\/$/, "")}/api/v2/support`;
}

function nativeRequest<T>(args: {
  url: string;
  method?: "GET" | "POST";
  data?: Record<string, unknown>;
}) {
  const session = getSession();
  return new Promise<T>((resolve, reject) => {
    uni.request({
      url: `${supportBase()}${args.url}`,
      method: args.method || "GET",
      data: args.data,
      header: {
        "Content-Type": "application/json",
        "X-User-ID": session.uid,
        Authorization: `Bearer ${session.token}`
      },
      success(response) {
        const envelope = (response.data || {}) as NativeEnvelope<T>;
        if (response.statusCode < 200 || response.statusCode >= 300 || Number(envelope.code || 0) !== 0) {
          reject(new Error(envelope.message || "客服服务请求失败"));
          return;
        }
        resolve(envelope.data as T);
      },
      fail(error) {
        reject(new Error(error.errMsg || "客服服务连接失败"));
      }
    });
  });
}

export function getSupportConversation() {
  return nativeRequest<SupportConversation>({ url: "/conversations/current" });
}

export async function getSupportMessages(conversationId: string) {
  const data = await nativeRequest<{ items?: SupportMessage[] }>({
    url: `/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`
  });
  return (data.items || []).slice().reverse();
}

export function sendSupportMessage(conversationId: string, text: string) {
  return nativeRequest<SupportMessage>({
    url: `/conversations/${encodeURIComponent(conversationId)}/messages`,
    method: "POST",
    data: {
      client_message_id: `support_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`,
      message_type: 1,
      text_content: text,
      asset_id: 0
    }
  });
}
