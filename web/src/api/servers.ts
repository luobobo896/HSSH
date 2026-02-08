import axios from 'axios';
import { Server } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

const client = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

export async function listServers(): Promise<Server[]> {
  const response = await client.get('/servers');
  return response.data;
}

export async function addServer(server: Omit<Server, 'id'>): Promise<Server> {
  const response = await client.post('/servers', server);
  return response.data;
}

export async function updateServer(id: string, server: Partial<Server>): Promise<Server> {
  const response = await client.put(`/servers/${id}`, server);
  return response.data;
}

export async function deleteServer(id: string): Promise<void> {
  await client.delete(`/servers/${id}`);
}

export async function testConnection(id: string): Promise<{ success: boolean; latency_ms: number }> {
  const response = await client.post(`/servers/${id}/test`);
  return response.data;
}
