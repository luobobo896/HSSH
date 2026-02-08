import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Terminal } from './Terminal';

// Mock xterm
vi.mock('@xterm/xterm', () => ({
  Terminal: class MockTerminal {
    open = vi.fn();
    write = vi.fn();
    writeln = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    dispose = vi.fn();
    clear = vi.fn();
    resize = vi.fn();
    options = {};
  },
}));

describe('Terminal Component', () => {
  const mockServer = {
    name: 'test-server',
    host: '192.168.1.100',
    port: 22,
    user: 'root',
    auth_type: 'key' as const,
    key_path: '~/.ssh/id_rsa',
    server_type: 'external' as const,
  };

  const mockOnClose = vi.fn();
  const mockOnError = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders terminal container', () => {
    render(
      <Terminal
        server={mockServer}
        isOpen={true}
        onClose={mockOnClose}
        onError={mockOnError}
      />
    );

    expect(screen.getByTestId('terminal-container')).toBeInTheDocument();
    expect(screen.getByText('test-server')).toBeInTheDocument();
    expect(screen.getByText('root@192.168.1.100:22')).toBeInTheDocument();
  });

  it('does not render when isOpen is false', () => {
    render(
      <Terminal
        server={mockServer}
        isOpen={false}
        onClose={mockOnClose}
        onError={mockOnError}
      />
    );

    expect(screen.queryByTestId('terminal-container')).not.toBeInTheDocument();
  });

  it('displays connection status', () => {
    render(
      <Terminal
        server={mockServer}
        isOpen={true}
        onClose={mockOnClose}
        onError={mockOnError}
      />
    );

    expect(screen.getByText('连接中...')).toBeInTheDocument();
  });

  it('handles internal server with gateway', () => {
    const internalServer = {
      ...mockServer,
      server_type: 'internal' as const,
      gateway: 'gateway-server',
    };

    render(
      <Terminal
        server={internalServer}
        isOpen={true}
        onClose={mockOnClose}
        onError={mockOnError}
      />
    );

    // Should show gateway indicator (text may be split across elements)
    expect(screen.getByText('gateway-server')).toBeInTheDocument();
    expect(screen.getByText('→')).toBeInTheDocument();
  });

  it('calls onClose when close button is clicked', () => {
    render(
      <Terminal
        server={mockServer}
        isOpen={true}
        onClose={mockOnClose}
        onError={mockOnError}
      />
    );

    const closeButton = screen.getByTitle('关闭');
    fireEvent.click(closeButton);

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });
});
