import axios from 'axios';
import { ProxyInfo, TransferProgress } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

const client = axios.create({
  baseURL: API_BASE,
});

export async function uploadFile(
  file: File,
  targetPath: string,
  targetHost: string,
  via?: string[]
): Promise<string> {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('target_path', targetPath);
  formData.append('target_host', targetHost);
  if (via && via.length > 0) {
    formData.append('via', via.join(','));
  }

  const response = await client.post('/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
  return response.data.task_id;
}

export async function uploadDirectory(
  files: File[],
  targetPath: string,
  targetHost: string,
  via?: string[]
): Promise<string> {
  const formData = new FormData();
  
  // 添加所有文件，保留相对路径
  files.forEach(file => {
    formData.append('files', file, file.webkitRelativePath || file.name);
  });
  
  formData.append('target_path', targetPath);
  formData.append('target_host', targetHost);
  formData.append('is_dir', 'true');
  
  if (via && via.length > 0) {
    formData.append('via', via.join(','));
  }

  const response = await client.post('/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
  return response.data.task_id;
}

export async function createProxy(
  localPort: number,
  remoteHost: string,
  remotePort: number,
  via?: string[]
): Promise<ProxyInfo> {
  const response = await client.post('/proxy', {
    local_addr: `:${localPort}`,
    remote_host: remoteHost,
    remote_port: remotePort,
    via,
  });
  return response.data;
}

export async function listProxies(): Promise<ProxyInfo[]> {
  const response = await client.get('/proxy');
  return response.data;
}

export async function deleteProxy(id: string): Promise<void> {
  await client.delete(`/proxy/${id}`);
}

export async function getProgress(taskId: string): Promise<TransferProgress> {
  const response = await client.get(`/ws/progress/${taskId}`);
  return response.data;
}

export interface DirEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size?: number;
}

export interface BrowseResponse {
  path: string;
  entries: DirEntry[];
  success: boolean;
  error?: string;
}

export async function browseDirectory(serverName: string, path: string): Promise<BrowseResponse> {
  const encodedPath = path.replace(/\//g, '%2F');
  const response = await client.get(`/browse/${serverName}/${encodedPath}`);
  return response.data;
}

export async function getCommonPaths(): Promise<string[]> {
  const response = await client.get('/browse/__common_paths__');
  return response.data.paths;
}
