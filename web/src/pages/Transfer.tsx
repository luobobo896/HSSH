import { useCallback, useEffect, useRef, useState } from 'react';
import { getProgress, uploadFile, uploadDirectory, browseDirectory, DirEntry } from '../api/transfer';
import { useServerStore } from '../stores/serverStore';
import { TransferProgress, Server } from '../types';

// 辅助函数：判断是否为内网服务器（支持数字和字符串格式）
const isInternalServer = (serverType: string | number | undefined): boolean => {
  return serverType === 'internal' || (serverType as unknown as number) === 1;
};

// 辅助函数：判断是否为外网服务器（支持数字和字符串格式）
const isExternalServer = (serverType: string | number | undefined): boolean => {
  return serverType === 'external' || (serverType as unknown as number) === 0;
};

// 辅助函数：通过服务器 ID 获取名称
const getServerNameById = (servers: Server[], id: string): string => {
  const server = servers.find(s => s.id === id);
  return server?.name || id.slice(0, 8) + '...';
};

// 辅助函数：通过网关 ID 获取网关名称
const getGatewayName = (servers: Server[], gatewayId: string | undefined): string => {
  if (!gatewayId) return '未配置';
  const gateway = servers.find(s => s.id === gatewayId);
  return gateway?.name || gatewayId.slice(0, 8) + '...';
};

const COMMON_PATHS = [
  { value: '/tmp/', label: '/tmp/ - 临时目录' },
  { value: '/root/', label: '/root/ - root用户目录' },
  { value: '/home/', label: '/home/ - 用户目录' },
  { value: '/var/www/', label: '/var/www/ - Web目录' },
  { value: '/opt/', label: '/opt/ - 应用目录' },
  { value: '/data/', label: '/data/ - 数据目录' },
  { value: '/usr/local/', label: '/usr/local/ - 本地软件目录' },
  { value: 'custom', label: '📂 浏览服务器目录...' },
  { value: 'manual', label: '✏️ 手动输入路径...' },
];

export function Transfer() {
  const { servers, preselectedServer, clearPreselectedServer } = useServerStore();
  const [uploadType, setUploadType] = useState<'file' | 'folder'>('file');
  const [file, setFile] = useState<File | null>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [targetHost, setTargetHost] = useState('');
  const [targetPath, setTargetPath] = useState('/tmp/');
  const [customPath, setCustomPath] = useState('');
  const [pathMode, setPathMode] = useState<'common' | 'browse' | 'manual'>('common');
  const [browsePath, setBrowsePath] = useState('/');
  const [browseEntries, setBrowseEntries] = useState<DirEntry[]>([]);
  const [browsing, setBrowsing] = useState(false);
  const [browseError, setBrowseError] = useState('');
  const [viaHops, setViaHops] = useState<string[]>([]);
  const [progress, setProgress] = useState<TransferProgress | null>(null);
  const [uploading, setUploading] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [showPathDropdown, setShowPathDropdown] = useState(false);
  const pathDropdownRef = useRef<HTMLDivElement>(null);

  // 处理预选中服务器（从服务器卡片跳转过来）
  useEffect(() => {
    if (preselectedServer && servers.length > 0) {
      const server = servers.find(s => s.id === preselectedServer);
      if (server) {
        setTargetHost(server.id); // 使用 ID 作为 target
      }
      clearPreselectedServer();
    }
  }, [preselectedServer, servers, clearPreselectedServer]);

  // 当选择目标服务器时，自动处理网关逻辑
  useEffect(() => {
    // 通过 ID 或 host 查找服务器
    const server = servers.find(s => s.id === targetHost || s.host === targetHost);
    if (server) {
      setSelectedServer(server);

      // 清理 viaHops 中不属于当前服务器的网关（通过 ID 比较）
      const otherGateways = servers
        .filter(s => s.id !== server.id && isInternalServer(s.server_type) && s.gateway_id)
        .map(s => s.gateway_id!);
      const cleanedHops = viaHops.filter(h => !otherGateways.includes(h));

      // 如果是内网服务器，自动添加网关到viaHops末尾
      // 正确顺序：本地 → 中转节点(HK) → 网关 → 内网服务器
      if (isInternalServer(server.server_type) && server.gateway_id) {
        // 检查网关是否已经在列表中
        if (!cleanedHops.includes(server.gateway_id)) {
          setViaHops([...cleanedHops, server.gateway_id]);
        } else {
          setViaHops(cleanedHops);
        }
      } else {
        // 外网服务器不需要网关
        setViaHops(cleanedHops);
      }
    } else {
      setSelectedServer(null);
      setViaHops([]); // 清空中转节点
    }
  }, [targetHost, servers]);

  // 点击外部关闭下拉框
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (pathDropdownRef.current && !pathDropdownRef.current.contains(event.target as Node)) {
        setShowPathDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
    
    if (uploadType === 'file') {
      const droppedFile = e.dataTransfer.files[0];
      if (droppedFile) setFile(droppedFile);
    } else {
      const droppedFiles = Array.from(e.dataTransfer.files);
      if (droppedFiles.length > 0) {
        setFiles(droppedFiles);
        // 从第一个文件的路径推断文件夹名称
        const relativePath = droppedFiles[0].webkitRelativePath;
        if (relativePath) {
          const folderName = relativePath.split('/')[0];
          setFile(new File([], folderName));
        }
      }
    }
  }, [uploadType]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
  }, []);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (uploadType === 'file') {
      const selectedFile = e.target.files?.[0];
      if (selectedFile) setFile(selectedFile);
    } else {
      const selectedFiles = Array.from(e.target.files || []);
      if (selectedFiles.length > 0) {
        setFiles(selectedFiles);
        // 从第一个文件的路径推断文件夹名称
        const relativePath = selectedFiles[0].webkitRelativePath;
        if (relativePath) {
          const folderName = relativePath.split('/')[0];
          setFile(new File([], folderName));
        }
      }
    }
  };

  const handlePathChange = (value: string) => {
    if (value === 'manual') {
      setPathMode('manual');
      setCustomPath(targetPath);
      setShowPathDropdown(false);
    } else if (value === 'custom') {
      setPathMode('browse');
      setBrowsePath('/');
      setShowPathDropdown(false);
      if (targetHost) {
        loadDirectory('/');
      }
    } else {
      setPathMode('common');
      setTargetPath(value);
      setShowPathDropdown(false);
    }
  };

  const loadDirectory = async (path: string) => {
    if (!targetHost) return;
    
    setBrowsing(true);
    setBrowseError('');
    try {
      const result = await browseDirectory(targetHost, path);
      if (result.success) {
        setBrowseEntries(result.entries);
        setBrowsePath(result.path);
        setTargetPath(result.path);
      } else {
        setBrowseError(result.error || '无法读取目录');
      }
    } catch (err) {
      setBrowseError('浏览目录失败: ' + (err as Error).message);
    } finally {
      setBrowsing(false);
    }
  };

  const navigateToEntry = (entry: DirEntry) => {
    if (entry.is_dir) {
      loadDirectory(entry.path);
    } else {
      // 选择文件，取其所在目录
      const dirPath = entry.path.substring(0, entry.path.lastIndexOf('/') + 1) || '/';
      setTargetPath(dirPath);
      setPathMode('common');
    }
  };

  const navigateUp = () => {
    if (browsePath === '/') return;
    const parentPath = browsePath.replace(/\/$/, '').split('/').slice(0, -1).join('/') || '/';
    loadDirectory(parentPath);
  };

  const handleCustomPathChange = (value: string) => {
    setCustomPath(value);
    setTargetPath(value);
  };

  const confirmManualPath = () => {
    setPathMode('common');
  };

  const cancelManualPath = () => {
    setPathMode('common');
    setTargetPath('/tmp/');
    setCustomPath('');
  };

  const handleUpload = async () => {
    if ((!file && files.length === 0) || !targetHost) return;

    setUploading(true);
    setProgress(null);
    
    try {
      let taskId: string;
      
      if (uploadType === 'file' && file) {
        taskId = await uploadFile(file, targetPath, targetHost, viaHops);
      } else if (uploadType === 'folder' && files.length > 0) {
        taskId = await uploadDirectory(files, targetPath, targetHost, viaHops);
      } else {
        throw new Error('没有选择文件或文件夹');
      }

      const pollProgress = async () => {
        try {
          const data = await getProgress(taskId);
          setProgress(data);
          if (data.status === 'completed' || data.status === 'failed') {
            setUploading(false);
          } else {
            setTimeout(pollProgress, 1000);
          }
        } catch (err) {
          console.error('Failed to get progress:', err);
          setUploading(false);
        }
      };

      pollProgress();
    } catch (err) {
      console.error('Upload failed:', err);
      setUploading(false);
    }
  };

  const toggleHop = (hopId: string) => {
    // 如果这是内网服务器的自动添加的网关，不允许取消
    if (selectedServer && isInternalServer(selectedServer.server_type) && hopId === selectedServer.gateway_id) {
      return;
    }

    setViaHops(prev =>
      prev.includes(hopId)
        ? prev.filter(h => h !== hopId)
        : [...prev, hopId]
    );
  };

  const clearSelection = () => {
    setFile(null);
    setFiles([]);
  };

  const formatSpeed = (bytesPerSec: number) => {
    if (bytesPerSec > 1024 * 1024) {
      return `${(bytesPerSec / 1024 / 1024).toFixed(2)} MB/s`;
    }
    return `${(bytesPerSec / 1024).toFixed(2)} KB/s`;
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    if (bytes > 1024 * 1024 * 1024) {
      return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
    }
    if (bytes > 1024 * 1024) {
      return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
    }
    return `${(bytes / 1024).toFixed(2)} KB`;
  };

  const getTotalSize = () => {
    if (uploadType === 'file' && file) {
      return file.size;
    }
    return files.reduce((total, f) => total + f.size, 0);
  };

  const getItemCount = () => {
    if (uploadType === 'file') {
      return file ? 1 : 0;
    }
    return files.length;
  };

  const getPathDisplayLabel = () => {
    if (pathMode === 'manual') {
      return customPath || '手动输入路径...';
    }
    if (pathMode === 'browse') {
      return browsePath;
    }
    const found = COMMON_PATHS.find(p => p.value === targetPath);
    return found ? found.label : targetPath;
  };

  return (
    <div className="space-y-5 animate-fade-in-up">
      {/* Header */}
      <div>
        <h1 className="text-xl font-semibold text-primary">文件传输</h1>
        <p className="text-tertiary text-sm mt-0.5">安全快速地在服务器之间传输文件</p>
      </div>

      {/* Upload Type Toggle */}
      <div className="flex gap-2">
        <button
          onClick={() => { setUploadType('file'); clearSelection(); }}
          className={`glass-option-card ${uploadType === 'file' ? 'selected' : ''}`}
        >
          <span className="glass-option-card-icon">📄</span>
          <span className="glass-option-card-title">上传文件</span>
        </button>
        <button
          onClick={() => { setUploadType('folder'); clearSelection(); }}
          className={`glass-option-card ${uploadType === 'folder' ? 'selected' : ''}`}
        >
          <span className="glass-option-card-icon">📁</span>
          <span className="glass-option-card-title">上传文件夹</span>
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Upload Area */}
        <div className="space-y-5">
          {/* File Drop Zone */}
          <div
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            className={`glass-card glass-card-interactive text-center ${
              isDragOver ? 'border-info-border bg-info-light' : ''
            } ${(file || files.length > 0) ? 'border-success-border bg-success-light' : ''}`}
          >
            {(file || files.length > 0) ? (
              <div>
                <div className="w-12 h-12 mx-auto mb-3 rounded-xl bg-success-light border border-success-border flex items-center justify-center">
                  <svg width="24" height="24" className="text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <p className="text-base font-medium text-primary mb-0.5">
                  {uploadType === 'file' ? file?.name : file?.name || '文件夹'}
                </p>
                <p className="text-sm text-tertiary">
                  {formatSize(getTotalSize())} · {getItemCount()} 个项目
                </p>
                {uploadType === 'folder' && files.length > 0 && (
                  <p className="text-xs text-quaternary mt-1">
                    包含 {files.filter(f => !f.name.includes('.')).length} 个文件夹, {files.filter(f => f.name.includes('.')).length} 个文件
                  </p>
                )}
                <button
                  onClick={clearSelection}
                  className="mt-3 glass-button glass-button-danger glass-button-sm"
                >
                  重新选择
                </button>
              </div>
            ) : (
              <div>
                <div className={`w-14 h-14 mx-auto mb-3 rounded-xl bg-glass flex items-center justify-center transition-transform ${isDragOver ? 'scale-105' : ''}`}>
                  <svg width="28" height="28" className="text-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                  </svg>
                </div>
                <p className="text-secondary text-base mb-1">
                  {uploadType === 'file' ? '拖拽文件到此处' : '拖拽文件夹到此处'}
                </p>
                <p className="text-quaternary text-sm mb-3">
                  {uploadType === 'file' ? '或者点击选择文件' : '或者点击选择文件夹'}
                </p>
                <input
                  type="file"
                  {...(uploadType === 'folder' ? { webkitdirectory: 'true', directory: 'true' } : {})}
                  onChange={handleFileSelect}
                  className="hidden"
                  id="file-input"
                />
                <label
                  htmlFor="file-input"
                  className="glass-button cursor-pointer inline-flex"
                >
                  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                  {uploadType === 'file' ? '选择文件' : '选择文件夹'}
                </label>
              </div>
            )}
          </div>

          {/* Target Configuration */}
          <div className="glass-card space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-info-light flex items-center justify-center">
                <svg className="w-4 h-4 text-info" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
              </div>
              <h3 className="text-base font-medium text-primary">目标配置</h3>
            </div>

            <div>
              <label className="glass-label">目标服务器</label>
              <select
                value={targetHost}
                onChange={(e) => setTargetHost(e.target.value)}
                className="glass-select"
              >
                <option value="">选择目标服务器</option>
                {servers.map(s => (
                  <option key={s.name} value={s.name}>
                    {isInternalServer(s.server_type) ? '🔒' : '🌐'} {s.name} ({s.host})
                  </option>
                ))}
              </select>
              {selectedServer && isInternalServer(selectedServer.server_type) && (
                <p className="text-xs text-warning-text mt-1">
                  内网服务器，将通过网关 {getGatewayName(servers, selectedServer.gateway_id)} 访问
                </p>
              )}
            </div>

            <div>
              <label className="glass-label">目标路径</label>
              
              {/* Path Selector Dropdown */}
              <div className="relative" ref={pathDropdownRef}>
                <button
                  onClick={() => setShowPathDropdown(!showPathDropdown)}
                  disabled={!targetHost}
                  className="w-full glass-select text-left flex items-center justify-between disabled:opacity-50"
                >
                  <span className={targetHost ? 'text-primary' : 'text-quaternary'}>
                    {targetHost ? getPathDisplayLabel() : '请先选择服务器'}
                  </span>
                  <svg 
                    width="16" 
                    height="16" 
                    className={`text-quaternary transition-transform ${showPathDropdown ? 'rotate-180' : ''}`}
                    fill="none" 
                    stroke="currentColor" 
                    viewBox="0 0 24 24"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </button>

                {/* Dropdown Menu */}
                {showPathDropdown && (
                  <div className="absolute z-50 w-full mt-1 bg-[#1a1a2e] border border-white/10 rounded-lg shadow-glass max-h-64 overflow-y-auto">
                    {COMMON_PATHS.map(path => (
                      <button
                        key={path.value}
                        onClick={() => handlePathChange(path.value)}
                        className="w-full text-left px-3 py-2 text-sm text-secondary hover:bg-white/5 transition-colors"
                      >
                        {path.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {/* Browse Mode */}
              {pathMode === 'browse' && targetHost && (
                <div className="mt-3 p-3 bg-white/5 rounded-lg border border-white/10">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs text-secondary">当前路径:</span>
                    <div className="flex gap-1">
                      <button
                        onClick={navigateUp}
                        disabled={browsePath === '/' || browsing}
                        className="glass-button glass-button-sm"
                      >
                        ↑ 上级
                      </button>
                      <button
                        onClick={() => { setPathMode('common'); setTargetPath(browsePath); }}
                        className="glass-button glass-button-sm glass-button-secondary"
                      >
                        ✓ 选择
                      </button>
                    </div>
                  </div>
                  <div className="text-sm text-primary font-mono bg-black/30 px-2 py-1 rounded mb-2 truncate">
                    {browsePath}
                  </div>
                  
                  {browsing ? (
                    <div className="flex items-center justify-center py-4">
                      <div className="w-5 h-5 border-2 border-info-border border-t-info rounded-full animate-spin mr-2" />
                      <span className="text-xs text-secondary">加载中...</span>
                    </div>
                  ) : browseError ? (
                    <div className="text-xs text-error-text py-2">{browseError}</div>
                  ) : (
                    <div className="max-h-40 overflow-y-auto space-y-0.5">
                      {browseEntries.map(entry => (
                        <button
                          key={entry.path}
                          onClick={() => navigateToEntry(entry)}
                          className="w-full text-left px-2 py-1.5 text-xs text-secondary hover:bg-white/10 rounded flex items-center gap-2"
                        >
                          <span>{entry.is_dir ? '📁' : '📄'}</span>
                          <span className="truncate">{entry.name}</span>
                          {entry.is_dir && <span className="text-quaternary ml-auto">→</span>}
                        </button>
                      ))}
                      {browseEntries.length === 0 && (
                        <div className="text-xs text-quaternary py-2 text-center">空目录</div>
                      )}
                    </div>
                  )}
                </div>
              )}

              {/* Manual Input Mode */}
              {pathMode === 'manual' && (
                <div className="mt-3 flex gap-2">
                  <input
                    type="text"
                    value={customPath}
                    onChange={(e) => handleCustomPathChange(e.target.value)}
                    className="glass-input flex-1"
                    placeholder="输入完整路径，如 /data/backup/"
                    autoFocus
                  />
                  <button
                    onClick={confirmManualPath}
                    className="glass-button px-3 bg-info-light text-info-text"
                  >
                    ✓
                  </button>
                  <button
                    onClick={cancelManualPath}
                    className="glass-button px-3"
                  >
                    ✕
                  </button>
                </div>
              )}

              {/* Path Preview */}
              <div className="mt-2 text-xs text-quaternary">
                上传路径: <span className="text-secondary font-mono">{targetHost ? `${targetHost}:${targetPath}` : '未选择'}</span>
              </div>
            </div>

            <button
              onClick={handleUpload}
              disabled={(!file && files.length === 0) || !targetHost || uploading}
              className="w-full glass-button glass-button-primary disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {uploading ? (
                <>
                  <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  上传中...
                </>
              ) : (
                <>
                  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                  </svg>
                  开始{uploadType === 'file' ? '上传' : '上传文件夹'}
                </>
              )}
            </button>
          </div>
        </div>

        {/* Hop Configuration */}
        <div className="space-y-5">
          {/* Hop Selection */}
          <div className="glass-card">
            <div className="flex items-center gap-2 mb-3">
              <div className="w-7 h-7 rounded-md bg-brand-secondary/20 flex items-center justify-center">
                <svg width="14" height="14" className="text-brand-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <h3 className="text-base font-medium text-primary">中转节点（可选）</h3>
            </div>
            <p className="text-tertiary text-xs mb-3">选择额外的跳板机以优化传输路径</p>

            {/* 获取可用的中转节点（只显示外网服务器，排除当前服务器的网关） */}
            {(() => {
              const availableHops = servers.filter(s =>
                isExternalServer(s.server_type) &&
                s.id !== selectedServer?.id &&
                s.id !== selectedServer?.gateway_id
              );
              
              if (availableHops.length === 0) {
                return (
                  <div className="text-center py-6 text-quaternary">
                    <div className="w-10 h-10 mx-auto mb-2 rounded-lg bg-white/5 flex items-center justify-center">
                      <svg width="20" height="20" className="text-quaternary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                      </svg>
                    </div>
                    <p className="text-sm">无可用的中转节点</p>
                  </div>
                );
              }
              
              return (
                <div className="space-y-1.5 max-h-48 overflow-y-auto">
                  {availableHops.map(server => (
                    <label
                      key={server.name}
                      className={`flex items-center gap-2.5 p-2.5 rounded-lg cursor-pointer transition-all ${
                        viaHops.includes(server.name)
                          ? 'bg-info-light border border-info-border'
                          : 'hover:bg-white/5 border border-transparent'
                      }`}
                    >
                      <div className="relative">
                        <input
                          type="checkbox"
                          checked={viaHops.includes(server.name)}
                          onChange={() => toggleHop(server.name)}
                          className="w-4 h-4 rounded border-2 border-white/30 bg-white/10 checked:bg-info checked:border-info appearance-none cursor-pointer transition-colors"
                        />
                        {viaHops.includes(server.name) && (
                          <svg width="10" height="10" className="text-white absolute top-0.5 left-0.5 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                          </svg>
                        )}
                      </div>
                      <div className="flex-1">
                        <span className="text-primary text-sm font-medium">{server.name}</span>
                        <span className="text-quaternary text-xs ml-1.5">({server.host})</span>
                      </div>
                    </label>
                  ))}
                </div>
              );
            })()}
            
            {/* 连接路径可视化 */}
            {selectedServer && (
              <div className="mt-3 p-2.5 bg-white/5 border border-white/10 rounded-lg">
                <div className="text-xs text-tertiary mb-1.5">连接路径</div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-xs bg-success-light text-success-text border border-success-border px-1.5 py-0.5 rounded">本地</span>
                  {/* 先显示中转节点(HK) */}
                  {viaHops.filter(h => h !== selectedServer.gateway_id).map(hopId => (
                    <span key={hopId} className="flex items-center gap-1.5">
                      <span className="text-quaternary">→</span>
                      <span className="text-xs bg-info-light text-info-text border border-info-border px-1.5 py-0.5 rounded">{getServerNameById(servers, hopId)}</span>
                    </span>
                  ))}
                  {/* 再显示网关 */}
                  {isInternalServer(selectedServer.server_type) && selectedServer.gateway_id && (
                    <>
                      <span className="text-quaternary">→</span>
                      <span className="text-xs bg-warning-light text-warning-text border border-warning-border px-1.5 py-0.5 rounded">
                        {getGatewayName(servers, selectedServer.gateway_id)}
                      </span>
                    </>
                  )}
                  <span className="text-quaternary">→</span>
                  <span className={`text-xs px-1.5 py-0.5 rounded border ${
                    isInternalServer(selectedServer.server_type)
                      ? 'bg-yellow-400/20 text-yellow-400'
                      : 'bg-blue-400/20 text-blue-400'
                  }`}>
                    {selectedServer.name}
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Transfer Progress */}
          {progress && (
            <div className="glass-card animate-fade-in-up">
              <div className="flex items-center gap-2 mb-3">
                <div className="w-7 h-7 rounded-md bg-green-400/20 flex items-center justify-center">
                  <svg width="14" height="14" className="text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                  </svg>
                </div>
                <h3 className="text-base font-medium text-primary">传输进度</h3>
              </div>

              <div className="space-y-3">
                <div className="flex justify-between items-center">
                  <span className="text-secondary text-sm truncate max-w-[60%]">{progress.file_name}</span>
                  <span className={`text-sm font-medium ${
                    progress.status === 'completed' ? 'text-green-400' :
                    progress.status === 'failed' ? 'text-red-400' :
                    'text-info-text'
                  }`}>
                    {progress.status === 'completed' ? '完成' :
                     progress.status === 'failed' ? '失败' :
                     `${progress.percentage.toFixed(1)}%`}
                  </span>
                </div>

                {/* Progress Bar */}
                <div className="glass-progress">
                  <div
                    className={`glass-progress-bar ${
                      progress.status === 'completed' ? 'glass-progress-bar-success' :
                      progress.status === 'failed' ? 'glass-progress-bar-error' :
                      'glass-progress-bar-info'
                    }`}
                    style={{ width: `${progress.percentage}%` }}
                  />
                </div>

                {progress.status === 'running' && (
                  <div className="grid grid-cols-2 gap-3">
                    <div className="glass-card glass-card-flat p-3 text-center">
                      <p className="text-quaternary text-xs mb-0.5">速度</p>
                      <p className="text-primary text-sm font-medium">{formatSpeed(progress.speed_bytes_per_sec)}</p>
                    </div>
                    <div className="glass-card glass-card-flat p-3 text-center">
                      <p className="text-quaternary text-xs mb-0.5">剩余</p>
                      <p className="text-primary text-sm font-medium">{Math.ceil(progress.eta_seconds)}s</p>
                    </div>
                  </div>
                )}
                
                {progress.error && (
                  <div className="p-3 bg-error-light border border-error-border rounded-lg">
                    <p className="text-error-text text-sm">{progress.error}</p>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
