export interface RemoteHostStatus {
  available: boolean;
  configured: boolean;
  running: boolean;
  device_code?: string;
  service_status: string;
  message?: string;
  permissions: Record<string, boolean>;
}

export type RemoteHostCallback = (status: RemoteHostStatus) => void;

export function initialize(options: Record<string, unknown>, callback: RemoteHostCallback): void;
export function start(callback: RemoteHostCallback): void;
export function stop(options: { clear_credentials?: boolean }, callback: RemoteHostCallback): void;
export function getStatus(callback: RemoteHostCallback): void;
export function openPermissionSettings(permission: string, callback: RemoteHostCallback): void;
