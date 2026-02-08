import { useEffect, useRef, useState, useCallback } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { Server } from '../../types';

interface TerminalProps {
  server: Server;
  isOpen: boolean;
  onClose: () => void;
  onError?: (error: string) => void;
}

interface TerminalMessage {
  type: 'output' | 'status' | 'error';
  data: string;
}

interface Position {
  x: number;
  y: number;
}

interface Size {
  width: number;
  height: number;
}

export function Terminal({ server, isOpen, onClose, onError }: TerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const modalRef = useRef<HTMLDivElement>(null);
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'error' | 'closed'>('connecting');
  const [errorMessage, setErrorMessage] = useState<string>('');

  // 窗口位置和大小状态
  const [position, setPosition] = useState<Position>({ x: 0, y: 0 });
  const [size, setSize] = useState<Size>({ width: 900, height: 600 });
  const [isMaximized, setIsMaximized] = useState(false);
  const [prevState, setPrevState] = useState<{ position: Position; size: Size } | null>(null);

  // 拖拽状态
  const dragRef = useRef<{ isDragging: boolean; startX: number; startY: number; startLeft: number; startTop: number }>({
    isDragging: false,
    startX: 0,
    startY: 0,
    startLeft: 0,
    startTop: 0,
  });

  // 调整大小状态
  const resizeRef = useRef<{ isResizing: boolean; startX: number; startY: number; startWidth: number; startHeight: number }>({
    isResizing: false,
    startX: 0,
    startY: 0,
    startWidth: 0,
    startHeight: 0,
  });

  // 获取 WebSocket URL
  const getWebSocketUrl = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/api/terminal?server=${encodeURIComponent(server?.name || '')}`;
  }, [server?.name]);

  // 初始化窗口位置（居中）
  useEffect(() => {
    if (isOpen && position.x === 0 && position.y === 0) {
      const centerX = Math.max(0, (window.innerWidth - size.width) / 2);
      const centerY = Math.max(0, (window.innerHeight - size.height) / 2);
      setPosition({ x: centerX, y: centerY });
    }
  }, [isOpen, size.width, size.height]);

  // 初始化终端
  useEffect(() => {
    if (!isOpen || !server || !terminalRef.current) return;

    // 重置连接状态
    setConnectionStatus('connecting');
    setErrorMessage('');

    // 创建 xterm 实例
    const term = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1a1a2e',
        foreground: '#eaeaea',
        cursor: '#eaeaea',
        selectionBackground: '#264f78',
        black: '#1a1a2e',
        red: '#ff5f56',
        green: '#27c93f',
        yellow: '#ffbd2e',
        blue: '#58a6ff',
        magenta: '#bc8cff',
        cyan: '#39c5cf',
        white: '#eaeaea',
        brightBlack: '#666666',
        brightRed: '#ff6b6b',
        brightGreen: '#5af78e',
        brightYellow: '#f1fa8c',
        brightBlue: '#79c0ff',
        brightMagenta: '#d2a8ff',
        brightCyan: '#56d4dd',
        brightWhite: '#ffffff',
      },
      scrollback: 10000,
      rows: 24,
      cols: 80,
    });

    // 打开终端
    term.open(terminalRef.current);
    xtermRef.current = term;

    // 使终端可交互 - 获取焦点
    term.focus();

    // 写入欢迎信息
    term.writeln('\r\n\x1b[36m╔══════════════════════════════════════════════════════════════╗\x1b[0m');
    term.writeln(`\x1b[36m║\x1b[0m  🖥️  正在连接到 \x1b[33m${server.name}\x1b[0m...` + ' '.repeat(Math.max(0, 32 - server.name.length)) + `\x1b[36m║\x1b[0m`);
    term.writeln(`\x1b[36m║\x1b[0m  👤 用户: \x1b[33m${server.user}\x1b[0m@${server.host}:${server.port}` + ' '.repeat(Math.max(0, 26 - server.user.length - server.host.length)) + `\x1b[36m║\x1b[0m`);

    if (server.server_type === 'internal' && server.gateway_id) {
      const gatewayDisplay = server.gateway_name || server.gateway_id.slice(0, 8) + '...';
      term.writeln(`\x1b[36m║\x1b[0m  🔗 网关: \x1b[33m${gatewayDisplay}\x1b[0m` + ' '.repeat(Math.max(0, 45 - gatewayDisplay.length)) + `\x1b[36m║\x1b[0m`);
    }

    term.writeln('\x1b[36m╚══════════════════════════════════════════════════════════════╝\x1b[0m\r\n');

    // 建立 WebSocket 连接
    const wsUrl = getWebSocketUrl();
    console.log('[Terminal] Connecting to:', wsUrl);

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('[Terminal] WebSocket connected');
      setConnectionStatus('connecting');
    };

    ws.onmessage = (event) => {
      try {
        const message: TerminalMessage = JSON.parse(event.data);

        switch (message.type) {
          case 'output':
            // 接收到的数据写入终端显示
            term.write(message.data);
            break;
          case 'status':
            if (message.data === 'connected') {
              setConnectionStatus('connected');
              term.writeln('\r\n\x1b[32m✓ 连接成功！可以开始输入命令\x1b[0m\r\n');
              // 连接成功后聚焦终端
              term.focus();
            } else if (message.data === 'disconnected') {
              setConnectionStatus('closed');
              term.writeln('\r\n\x1b[31m✗ 连接已断开\x1b[0m\r\n');
            }
            break;
          case 'error':
            setConnectionStatus('error');
            setErrorMessage(message.data);
            term.writeln(`\r\n\x1b[31m✗ 错误: ${message.data}\x1b[0m\r\n`);
            onError?.(message.data);
            break;
        }
      } catch (err) {
        console.error('[Terminal] Failed to parse message:', err);
      }
    };

    ws.onerror = (error) => {
      console.error('[Terminal] WebSocket error:', error);
      setConnectionStatus('error');
      setErrorMessage('WebSocket 连接错误');
      term.writeln('\r\n\x1b[31m✗ WebSocket 连接错误\x1b[0m\r\n');
      onError?.('WebSocket 连接错误');
    };

    ws.onclose = () => {
      console.log('[Terminal] WebSocket closed');
      setConnectionStatus('closed');
    };

    // 处理终端输入 - 用户输入发送到服务器
    const disposable = term.onData((data) => {
      console.log('[Terminal] Input received:', data, 'WebSocket state:', ws.readyState);
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      } else {
        console.warn('[Terminal] WebSocket not open, cannot send input');
      }
    });

    // 确保终端可点击获取焦点
    term.attachCustomKeyEventHandler((e) => {
      console.log('[Terminal] Key event:', e.type, e.key, 'ctrl:', e.ctrlKey, 'alt:', e.altKey);
      return true;
    });

    // 处理终端大小调整
    const handleResize = () => {
      if (terminalRef.current && xtermRef.current) {
        const { cols, rows } = xtermRef.current;
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'resize',
            data: JSON.stringify({ cols, rows }),
          }));
        }
      }
    };

    window.addEventListener('resize', handleResize);

    // 清理函数
    return () => {
      window.removeEventListener('resize', handleResize);
      disposable.dispose();

      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close();
      }

      term.dispose();
      xtermRef.current = null;
      wsRef.current = null;
    };
  }, [isOpen, server, getWebSocketUrl, onError]);

  // 调整大小时更新终端尺寸
  useEffect(() => {
    if (xtermRef.current && terminalRef.current) {
      // 使用 fitAddon 或手动计算行列数
      const terminalWidth = terminalRef.current.clientWidth;
      const terminalHeight = terminalRef.current.clientHeight;
      const cols = Math.floor(terminalWidth / 9);
      const rows = Math.floor(terminalHeight / 17);
      xtermRef.current.resize(Math.max(10, cols), Math.max(5, rows));
    }
  }, [size]);

  // 拖拽移动处理
  const handleMouseDown = (e: React.MouseEvent) => {
    if (isMaximized) return;
    const target = e.target as HTMLElement;
    if (target.closest('.no-drag')) return;

    dragRef.current = {
      isDragging: true,
      startX: e.clientX,
      startY: e.clientY,
      startLeft: position.x,
      startTop: position.y,
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!dragRef.current.isDragging) return;

    const dx = e.clientX - dragRef.current.startX;
    const dy = e.clientY - dragRef.current.startY;

    setPosition({
      x: Math.max(0, dragRef.current.startLeft + dx),
      y: Math.max(0, dragRef.current.startTop + dy),
    });
  };

  const handleMouseUp = () => {
    dragRef.current.isDragging = false;
    document.removeEventListener('mousemove', handleMouseMove);
    document.removeEventListener('mouseup', handleMouseUp);
  };

  // 调整大小处理
  const handleResizeStart = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isMaximized) return;

    resizeRef.current = {
      isResizing: true,
      startX: e.clientX,
      startY: e.clientY,
      startWidth: size.width,
      startHeight: size.height,
    };

    document.addEventListener('mousemove', handleResizeMove);
    document.addEventListener('mouseup', handleResizeUp);
  };

  const handleResizeMove = (e: MouseEvent) => {
    if (!resizeRef.current.isResizing) return;

    const dx = e.clientX - resizeRef.current.startX;
    const dy = e.clientY - resizeRef.current.startY;

    setSize({
      width: Math.max(400, resizeRef.current.startWidth + dx),
      height: Math.max(300, resizeRef.current.startHeight + dy),
    });
  };

  const handleResizeUp = () => {
    resizeRef.current.isResizing = false;
    document.removeEventListener('mousemove', handleResizeMove);
    document.removeEventListener('mouseup', handleResizeUp);
  };

  // 最大化/还原
  const toggleMaximize = () => {
    if (isMaximized) {
      if (prevState) {
        setPosition(prevState.position);
        setSize(prevState.size);
      }
      setIsMaximized(false);
    } else {
      setPrevState({ position, size });
      setPosition({ x: 0, y: 0 });
      setSize({ width: window.innerWidth, height: window.innerHeight });
      setIsMaximized(true);
    }
  };

  // 获取状态显示
  const getStatusDisplay = () => {
    switch (connectionStatus) {
      case 'connecting':
        return { text: '连接中...', color: 'text-yellow-400', bg: 'bg-yellow-400/20' };
      case 'connected':
        return { text: '已连接', color: 'text-green-400', bg: 'bg-green-400/20' };
      case 'error':
        return { text: '连接失败', color: 'text-red-400', bg: 'bg-red-400/20' };
      case 'closed':
        return { text: '已断开', color: 'text-gray-400', bg: 'bg-gray-400/20' };
      default:
        return { text: '未知', color: 'text-gray-400', bg: 'bg-gray-400/20' };
    }
  };

  const status = getStatusDisplay();

  if (!isOpen || !server) return null;

  return (
    <>
      {/* 背景遮罩 */}
      <div className="fixed inset-0 bg-black/30 backdrop-blur-sm z-40" onClick={onClose} />

      {/* 终端窗口 */}
      <div
        ref={modalRef}
        className="fixed z-50 flex flex-col rounded-lg overflow-hidden"
        style={{
          left: position.x,
          top: position.y,
          width: size.width,
          height: size.height,
          background: '#1a1a2e',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.1)',
        }}
      >
        {/* 标题栏 - 可拖拽 */}
        <div
          className="h-10 flex items-center justify-between px-3 select-none cursor-move"
          style={{ background: 'linear-gradient(180deg, #2a2a3e 0%, #1f1f2e 100%)' }}
          onMouseDown={handleMouseDown}
        >
          {/* 左侧：窗口控制按钮 */}
          <div className="flex items-center gap-2 no-drag">
            <button
              onClick={onClose}
              className="w-3 h-3 rounded-full bg-[#ff5f56] hover:bg-[#ff5f56]/80 transition-colors flex items-center justify-center"
              title="关闭"
            >
              <svg className="w-2 h-2 text-black/60 opacity-0 hover:opacity-100" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            <button
              onClick={toggleMaximize}
              className="w-3 h-3 rounded-full bg-[#ffbd2e] hover:bg-[#ffbd2e]/80 transition-colors flex items-center justify-center"
              title={isMaximized ? "还原" : "最大化"}
            >
              <svg className="w-2 h-2 text-black/60 opacity-0 hover:opacity-100" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 4l-5-5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
            </button>
            <button
              className="w-3 h-3 rounded-full bg-[#27c93f] hover:bg-[#27c93f]/80 transition-colors flex items-center justify-center"
              title="最小化"
            >
              <svg className="w-2 h-2 text-black/60 opacity-0 hover:opacity-100" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M20 12H4" />
              </svg>
            </button>
          </div>

          {/* 中间：标题 */}
          <div className="flex items-center gap-2 text-sm text-gray-300 font-medium">
            <span>🖥️</span>
            <span>{server.name}</span>
            <span className="text-gray-500">-</span>
            <span className="text-gray-400">{server.user}@{server.host}:{server.port}</span>
          </div>

          {/* 右侧：状态和控制 */}
          <div className="flex items-center gap-2 no-drag">
            {/* 连接状态 */}
            <span className={`glass-badge ${
              connectionStatus === 'connected' ? 'glass-badge-success' :
              connectionStatus === 'error' ? 'glass-badge-error' :
              connectionStatus === 'connecting' ? 'glass-badge-warning' :
              'glass-badge-neutral'
            }`}>
              {status.text}
            </span>

            {/* 服务器类型 */}
            {server.server_type === 'internal' && server.gateway_id && (
              <div className="flex items-center gap-1 text-xs text-warning-text bg-warning-light border border-warning-border px-2 py-0.5 rounded">
                <span>→</span>
                <span>{server.gateway_name || server.gateway_id.slice(0, 8) + '...'}</span>
              </div>
            )}

            {/* 关闭按钮 */}
            <button
              onClick={onClose}
              className="ml-2 w-6 h-6 flex items-center justify-center rounded hover:bg-white/10 transition-colors"
              title="关闭"
            >
              <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* 错误消息 */}
        {errorMessage && (
          <div className="px-4 py-2 bg-red-400/10 border-b border-red-400/20">
            <p className="text-sm text-red-400">{errorMessage}</p>
          </div>
        )}

        {/* 终端容器 - 可交互区域 */}
        <div className="flex-1 relative overflow-hidden" style={{ background: '#1a1a2e' }}>
          <div
            ref={terminalRef}
            className="absolute inset-0 p-2"
            style={{ background: '#1a1a2e' }}
            onClick={() => xtermRef.current?.focus()}
          />
        </div>

        {/* 底部信息栏 */}
        <div
          className="h-7 flex items-center justify-between px-3 text-xs text-gray-500 border-t"
          style={{ background: '#1f1f2e', borderColor: 'rgba(255,255,255,0.05)' }}
        >
          <div className="flex items-center gap-4">
            <span>xterm-256color</span>
            <span>•</span>
            <span>UTF-8</span>
          </div>
          <div className="flex items-center gap-2">
            <span className={server.server_type === 'internal' ? 'text-yellow-400' : 'text-blue-400'}>
              {server.server_type === 'internal' ? '🔒 内网' : '🌐 外网'}
            </span>
          </div>
        </div>

        {/* 调整大小手柄 */}
        {!isMaximized && (
          <div
            className="absolute bottom-0 right-0 w-4 h-4 cursor-se-resize z-10"
            onMouseDown={handleResizeStart}
            style={{
              background: 'linear-gradient(135deg, transparent 50%, rgba(255,255,255,0.3) 50%)',
            }}
          />
        )}
      </div>
    </>
  );
}
