import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock WebSocket
class MockWebSocket {
  send = vi.fn();
  close = vi.fn();
  readyState = 1; // WebSocket.OPEN
  onopen: ((this: WebSocket, ev: Event) => any) | null = null;
  onmessage: ((this: WebSocket, ev: MessageEvent) => any) | null = null;
  onclose: ((this: WebSocket, ev: CloseEvent) => any) | null = null;
  onerror: ((this: WebSocket, ev: Event) => any) | null = null;
}

global.WebSocket = MockWebSocket as any;
