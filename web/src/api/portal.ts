import axios from 'axios';
import { PortMapping, PortalStatus } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

const client = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

export interface CreateMappingRequest {
  name: string;
  local_addr: string;
  remote_host: string;
  remote_port: number;
  via?: string[];
  protocol?: string;
}

export async function getPortalStatus(): Promise<PortalStatus> {
  const response = await client.get('/portal');
  return response.data;
}

export async function getMappings(): Promise<PortMapping[]> {
  const response = await client.get('/portal/mappings');
  return response.data;
}

export async function createMapping(request: CreateMappingRequest): Promise<PortMapping> {
  const response = await client.post('/portal/mappings', request);
  return response.data;
}

export async function deleteMapping(id: string): Promise<void> {
  await client.delete(`/portal/mappings/${id}`);
}
