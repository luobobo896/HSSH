import { create } from 'zustand';
import { Server } from '../types';
import * as api from '../api/servers';

interface ServerState {
  servers: Server[];
  loading: boolean;
  error: string | null;
  preselectedServer: string | null; // 预选中用于传输的服务器（现在存储 ID）
  fetchServers: () => Promise<void>;
  addServer: (server: Omit<Server, 'id'>) => Promise<void>;
  updateServer: (id: string, server: Partial<Server>) => Promise<void>;
  deleteServer: (id: string) => Promise<void>;
  getServerById: (id: string) => Server | undefined;
  getServerByName: (name: string) => Server | undefined;
  setPreselectedServer: (id: string | null) => void;
  clearPreselectedServer: () => void;
}

export const useServerStore = create<ServerState>((set, get) => ({
  servers: [],
  loading: false,
  error: null,
  preselectedServer: null,

  fetchServers: async () => {
    set({ loading: true, error: null });
    try {
      const servers = await api.listServers();
      console.log('[DEBUG] fetchServers - servers:', JSON.stringify(servers));
      set({ servers, loading: false });
    } catch (err) {
      set({ error: String(err), loading: false });
    }
  },

  addServer: async (server) => {
    set({ loading: true, error: null });
    try {
      await api.addServer(server);
      await get().fetchServers();
    } catch (err) {
      set({ error: String(err), loading: false });
    }
  },

  updateServer: async (id, server) => {
    set({ loading: true, error: null });
    console.log('[DEBUG] store.updateServer - id:', id, 'server:', JSON.stringify(server));
    try {
      const response = await api.updateServer(id, server);
      console.log('[DEBUG] store.updateServer - response:', JSON.stringify(response));
      await get().fetchServers();
    } catch (err) {
      console.error('[DEBUG] store.updateServer - error:', err);
      set({ error: String(err), loading: false });
    }
  },

  deleteServer: async (id) => {
    set({ loading: true, error: null });
    try {
      await api.deleteServer(id);
      await get().fetchServers();
    } catch (err) {
      set({ error: String(err), loading: false });
    }
  },

  getServerById: (id) => {
    return get().servers.find(s => s.id === id);
  },

  getServerByName: (name) => {
    return get().servers.find(s => s.name === name);
  },

  setPreselectedServer: (name) => {
    set({ preselectedServer: name });
  },

  clearPreselectedServer: () => {
    set({ preselectedServer: null });
  },
}));
