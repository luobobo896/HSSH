export type ServerType = 'external' | 'internal';

export interface Hop {
  id: string; // 唯一标识符 (UUID)
  name: string;
  host: string;
  port: number;
  user: string;
  auth_type: 'key' | 'password';
  key_path?: string;
  password?: string;
  server_type: ServerType;
  gateway_id?: string; // 网关服务器ID
  gateway_name?: string; // 网关显示名称（后端填充）
}

export interface RoutePreference {
  from_id: string;
  to_id: string;
  via_id?: string;
  from_name?: string; // 显示用
  to_name?: string;
  via_name?: string;
  threshold: number;
}

export interface Profile {
  id: string;
  name: string;
  path_ids: string[];
  path_names?: string[]; // 显示用
  target_dir?: string;
  local_port?: number;
  remote_host?: string;
  remote_port?: number;
}

export interface TransferProgress {
  task_id: string;
  file_name: string;
  total_bytes: number;
  sent_bytes: number;
  speed_bytes_per_sec: number;
  eta_seconds: number;
  status: 'pending' | 'running' | 'completed' | 'failed';
  error?: string;
  percentage: number;
}

export interface ProxyInfo {
  id: string;
  local_addr: string;
  remote_host: string;
  remote_port: number;
  active: boolean;
  connection_count: number;
}

export interface LatencyReport {
  path: Array<{
    id: string;
    name: string;
    host: string;
  }>;
  latency_ms: number;
  success: boolean;
  error?: string;
}

export type Server = Hop;

export type PortalProtocol = 'tcp' | 'http' | 'websocket';

export interface PortMapping {
  id: string;
  name: string;
  local_addr: string;
  remote_host: string;
  remote_port: number;
  via?: string[];
  protocol: PortalProtocol;
  enabled: boolean;
  active?: boolean;
  connection_count?: number;
  bytes_transferred?: number;
}

export interface PortalStatus {
  active: boolean;
  mappings: PortMapping[];
  server_addr?: string;
}
