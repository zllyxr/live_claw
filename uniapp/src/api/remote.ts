import { API_HOST } from "@/constants/config";
import { getSession } from "@/utils/session";

export interface RemotePermissionStatus {
  notification?: boolean;
  media_projection?: boolean;
  system_audio?: boolean;
  accessibility?: boolean;
  overlay?: boolean;
  all_files?: boolean;
  microphone?: boolean;
  battery?: boolean;
}

export interface RemoteDeviceStatus {
  id: string;
  device_code: string;
  service_status: string;
  online: boolean;
  permission_status?: RemotePermissionStatus;
  last_seen_at?: string;
}

export interface RemoteEnrollment {
  device_id: string;
  device_token: string;
  heartbeat_seconds: number;
}

interface V2Envelope<T> {
  code: number;
  message: string;
  data: T;
}

const V2_BASE = `${API_HOST}/api/v2`;

function requestV2<T>(path: string, options: {
  method?: "GET" | "POST" | "DELETE";
  data?: Record<string, unknown>;
  installId?: string;
  session?: { uid: string; token: string };
} = {}): Promise<T> {
  const session = options.session || getSession();
  return new Promise((resolve, reject) => {
    uni.request({
      url: `${V2_BASE}${path}`,
      method: options.method || "GET",
      timeout: 12000,
      data: options.data,
      header: {
        "Content-Type": "application/json",
        "X-User-ID": String(session.uid || ""),
        "Authorization": `Bearer ${String(session.token || "")}`,
        ...(options.installId ? { "X-Install-ID": options.installId } : {})
      },
      success: (response) => {
        const result = response.data as V2Envelope<T>;
        if (response.statusCode < 200 || response.statusCode >= 300 || Number(result?.code) !== 0) {
          reject(new Error(result?.message || "远程协助请求失败"));
          return;
        }
        resolve(result.data);
      },
      fail: (error) => reject(new Error(error.errMsg || "网络请求失败"))
    });
  });
}

export function enrollRemoteDevice(installId: string, device: Record<string, unknown>) {
  return requestV2<RemoteEnrollment>("/remote/devices/enroll", {
    method: "POST",
    data: { install_id: installId, ...device }
  });
}

export function getCurrentRemoteDevice(installId: string) {
  return requestV2<RemoteDeviceStatus>("/remote/devices/current", { installId });
}

export function unbindRemoteDevice(
  installId: string,
  session?: { uid: string; token: string }
) {
  return requestV2<{ unbound: boolean }>("/remote/devices/current", {
    method: "DELETE", installId, session
  });
}
