import { useEffect, useState } from 'react';
import { getMappings, createMapping, updateMapping, deleteMapping, startMapping, stopMapping, CreateMappingRequest } from '../api/portal';
import { PortMapping, PortalProtocol } from '../types';
import { useServerStore } from '../stores/serverStore';

const PROTOCOL_OPTIONS: { value: PortalProtocol; label: string; icon: string }[] = [
  { value: 'tcp', label: 'TCP', icon: '🔌' },
  { value: 'http', label: 'HTTP', icon: '🌐' },
  { value: 'websocket', label: 'WebSocket', icon: '🔵' },
];

export function Portal() {
  const { servers } = useServerStore();
  const [mappings, setMappings] = useState<PortMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingMapping, setEditingMapping] = useState<PortMapping | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const [newMapping, setNewMapping] = useState<Partial<CreateMappingRequest>>({
    local_addr: ':8080',
    remote_port: 80,
    protocol: 'tcp',
  });

  useEffect(() => {
    loadMappings();
  }, []);

  const loadMappings = async () => {
    try {
      setLoading(true);
      const data = await getMappings();
      setMappings(data);
    } catch (err) {
      console.error('Failed to load mappings:', err);
    } finally {
      setLoading(false);
    }
  };

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!newMapping.name?.trim()) {
      newErrors.name = '请输入映射名称';
    }

    if (!newMapping.local_addr?.trim()) {
      newErrors.local_addr = '请输入本地地址';
    } else {
      // 验证地址格式，支持 :port 或 host:port
      const addrPattern = /^([\w.]*:\d+|:\d+)$/;
      if (!addrPattern.test(newMapping.local_addr)) {
        newErrors.local_addr = '格式错误，应为 :port 或 host:port';
      }
    }

    if (!newMapping.remote_host?.trim()) {
      newErrors.remote_host = '请输入远程主机';
    }

    if (!newMapping.remote_port || newMapping.remote_port < 1 || newMapping.remote_port > 65535) {
      newErrors.remote_port = '端口范围 1-65535';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setSubmitting(true);
    try {
      if (editingMapping) {
        // Update existing mapping
        await updateMapping(editingMapping.id, newMapping);
        setEditingMapping(null);
      } else {
        // Create new mapping
        await createMapping(newMapping as CreateMappingRequest);
        setShowAddForm(false);
      }
      setNewMapping({
        local_addr: ':8080',
        remote_port: 80,
        protocol: 'tcp',
      });
      setErrors({});
      await loadMappings();
    } catch (err) {
      console.error('Failed to save mapping:', err);
    } finally {
      setSubmitting(false);
    }
  };

  const handleEdit = (mapping: PortMapping) => {
    setEditingMapping(mapping);
    setNewMapping({
      name: mapping.name,
      local_addr: mapping.local_addr,
      remote_host: mapping.remote_host,
      remote_port: mapping.remote_port,
      protocol: mapping.protocol,
      via: mapping.via,
      portal_server: mapping.portal_server,
    });
    setErrors({});
  };

  const handleCloseModal = () => {
    setShowAddForm(false);
    setEditingMapping(null);
    setNewMapping({
      local_addr: ':8080',
      remote_port: 80,
      protocol: 'tcp',
    });
    setErrors({});
  };

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个端口映射吗？')) {
      return;
    }

    try {
      await deleteMapping(id);
      await loadMappings();
    } catch (err) {
      console.error('Failed to delete mapping:', err);
    }
  };

  const handleStart = async (id: string) => {
    try {
      await startMapping(id);
      await loadMappings();
    } catch (err) {
      console.error('Failed to start mapping:', err);
      alert('启动失败: ' + (err instanceof Error ? err.message : '未知错误'));
    }
  };

  const handleStop = async (id: string) => {
    try {
      await stopMapping(id);
      await loadMappings();
    } catch (err) {
      console.error('Failed to stop mapping:', err);
      alert('停止失败: ' + (err instanceof Error ? err.message : '未知错误'));
    }
  };

  const getProtocolLabel = (protocol: PortalProtocol) => {
    return PROTOCOL_OPTIONS.find(p => p.value === protocol)?.label || protocol;
  };

  const getProtocolIcon = (protocol: PortalProtocol) => {
    return PROTOCOL_OPTIONS.find(p => p.value === protocol)?.icon || '🔌';
  };

  const getProtocolColor = (protocol: PortalProtocol) => {
    switch (protocol) {
      case 'tcp':
        return 'bg-info-light text-info-text border-info-border';
      case 'http':
        return 'bg-success-light text-success-text border-success-border';
      case 'websocket':
        return 'bg-brand-secondary/20 text-brand-secondary border-brand-secondary/30';
      default:
        return 'bg-glass text-secondary border-glass-border';
    }
  };

  const toggleViaHop = (hopId: string) => {
    setNewMapping(prev => {
      const currentVia = prev.via || [];
      const newVia = currentVia.includes(hopId)
        ? currentVia.filter(h => h !== hopId)
        : [...currentVia, hopId];
      return { ...prev, via: newVia };
    });
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div>
        <div className="mb-5">
          <h1 className="text-xl font-semibold text-primary">端口转发</h1>
          <p className="text-tertiary text-sm mt-2">管理本地到远程服务器的端口映射</p>
        </div>
        <button
          onClick={() => setShowAddForm(true)}
          className="glass-button glass-button-primary mb-6"
        >
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
          添加映射
        </button>
      </div>

      {/* Loading State */}
      {loading && (
        <div className="glass-loading">
          <div className="glass-spinner" />
          <p className="glass-loading-text">加载中...</p>
        </div>
      )}

      {/* Mappings Grid */}
      {!loading && (
        <>
          {mappings.length === 0 ? (
            <div className="glass-empty">
              <div className="glass-empty-icon">
                <svg width="32" height="32" className="text-quaternary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <p className="glass-empty-title">暂无端口映射</p>
              <p className="glass-empty-description">点击上方按钮添加第一个端口映射</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {mappings.map((mapping, index) => (
                <div
                  key={mapping.id}
                  className="glass-card p-5 group"
                  style={{ animationDelay: `${index * 0.1}s` }}
                >
                  {/* Card Header */}
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                      <div className={`w-12 h-12 rounded-xl flex items-center justify-center text-2xl ${getProtocolColor(mapping.protocol)}`}>
                        {getProtocolIcon(mapping.protocol)}
                      </div>
                      <div>
                        <h3 className="font-semibold text-primary text-lg">{mapping.name}</h3>
                        <p className="text-tertiary text-sm">{mapping.local_addr}</p>
                      </div>
                    </div>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={() => handleEdit(mapping)}
                        className="glass-button-icon-sm glass-button-secondary"
                        title="编辑"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                        </svg>
                      </button>
                      {mapping.active ? (
                        <button
                          onClick={() => handleStop(mapping.id)}
                          className="glass-button-icon-sm glass-button-danger"
                          title="停止"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
                          </svg>
                        </button>
                      ) : (
                        <button
                          onClick={() => handleStart(mapping.id)}
                          className="glass-button-icon-sm glass-button-success"
                          title="启动"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                        </button>
                      )}
                      <button
                        onClick={() => handleDelete(mapping.id)}
                        className="glass-button-icon-sm glass-button-danger"
                        title="删除"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  </div>

                  {/* Mapping Details */}
                  <div className="space-y-2 mb-4">
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-tertiary">协议</span>
                      <span className={`glass-badge ${getProtocolColor(mapping.protocol)}`}>
                        {getProtocolIcon(mapping.protocol)} {getProtocolLabel(mapping.protocol)}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-tertiary">本地地址</span>
                      <span className="text-primary font-mono">{mapping.local_addr}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-tertiary">远程目标</span>
                      <span className="text-primary font-mono">{mapping.remote_host}:{mapping.remote_port}</span>
                    </div>
                    {mapping.via && mapping.via.length > 0 && (
                      <div className="flex justify-between text-sm">
                        <span className="text-tertiary">中转节点</span>
                        <span className="text-secondary">{mapping.via.join(' → ')}</span>
                      </div>
                    )}
                    {mapping.portal_server && (
                      <div className="flex justify-between text-sm">
                        <span className="text-tertiary">Portal 服务器</span>
                        <span className="text-primary font-mono">{mapping.portal_server}</span>
                      </div>
                    )}
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-tertiary">状态</span>
                      <span className={`glass-badge ${mapping.active ? 'glass-badge-green' : 'glass-badge-yellow'}`}>
                        {mapping.active ? '🟢 活跃' : '⏸️ 待机'}
                      </span>
                    </div>
                    {mapping.connection_count !== undefined && (
                      <div className="flex justify-between text-sm">
                        <span className="text-tertiary">连接数</span>
                        <span className="text-primary">{mapping.connection_count}</span>
                      </div>
                    )}
                  </div>

                  {/* Connection Path Visualization */}
                  {mapping.via && mapping.via.length > 0 && (
                    <div className="mt-3 p-2.5 bg-white/5 border border-white/10 rounded-lg">
                      <div className="text-xs text-tertiary mb-1.5">转发路径</div>
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="text-xs bg-info-light text-info-text border border-info-border px-1.5 py-0.5 rounded">本地</span>
                        {mapping.via.map((hop, idx) => (
                          <span key={idx} className="flex items-center gap-1.5">
                            <span className="text-quaternary">→</span>
                            <span className="text-xs bg-brand-secondary/20 text-brand-secondary border border-brand-secondary/30 px-1.5 py-0.5 rounded">{hop}</span>
                          </span>
                        ))}
                        <span className="text-quaternary">→</span>
                        <span className="text-xs bg-success-light text-success-text border border-success-border px-1.5 py-0.5 rounded">{mapping.remote_host}</span>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {/* Add/Edit Mapping Modal */}
      {(showAddForm || editingMapping) && (
        <div className="glass-modal-overlay">
          <div className="glass-modal glass-modal-lg animate-scale-in">
            {/* Modal Header */}
            <div className="glass-modal-header">
              <h2 className="glass-modal-title">
                {editingMapping ? '编辑端口映射' : '添加端口映射'}
              </h2>
              <button
                onClick={handleCloseModal}
                className="glass-modal-close"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <form onSubmit={handleSubmit} className="glass-modal-container">
              {/* Scrollable Body */}
              <div className="glass-modal-body space-y-4">
                {/* Protocol Selection */}
                <div>
                  <label className="glass-label">协议类型</label>
                  <div className="grid grid-cols-3 gap-3">
                    {PROTOCOL_OPTIONS.map((option) => (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => setNewMapping(prev => ({ ...prev, protocol: option.value }))}
                        className={`glass-option-card ${newMapping.protocol === option.value ? 'selected' : ''}`}
                      >
                        <span className="glass-option-card-icon">{option.icon}</span>
                        <span className="glass-option-card-title">{option.label}</span>
                      </button>
                    ))}
                  </div>
                </div>

              {/* Basic Info */}
              <div className="space-y-3">
                <div>
                  <label className="glass-label">名称</label>
                  <input
                    type="text"
                    value={newMapping.name || ''}
                    onChange={(e) => {
                      setNewMapping(prev => ({ ...prev, name: e.target.value }));
                      setErrors(prev => ({ ...prev, name: '' }));
                    }}
                    className={`glass-input ${errors.name ? 'border-red-400/50' : ''}`}
                    placeholder="例如: web-server, mysql-proxy"
                  />
                  {errors.name && <p className="glass-error-text">{errors.name}</p>}
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="glass-label">本地地址</label>
                    <input
                      type="text"
                      value={newMapping.local_addr || ''}
                      onChange={(e) => {
                        setNewMapping(prev => ({ ...prev, local_addr: e.target.value }));
                        setErrors(prev => ({ ...prev, local_addr: '' }));
                      }}
                      className={`glass-input ${errors.local_addr ? 'border-red-400/50' : ''}`}
                      placeholder=":8080 或 127.0.0.1:8080"
                    />
                    {errors.local_addr && <p className="glass-error-text">{errors.local_addr}</p>}
                  </div>
                  <div>
                    <label className="glass-label">远程端口</label>
                    <input
                      type="number"
                      value={newMapping.remote_port || ''}
                      onChange={(e) => {
                        setNewMapping(prev => ({ ...prev, remote_port: parseInt(e.target.value) || 0 }));
                        setErrors(prev => ({ ...prev, remote_port: '' }));
                      }}
                      className={`glass-input ${errors.remote_port ? 'border-red-400/50' : ''}`}
                      placeholder="80, 3306, etc"
                      min={1}
                      max={65535}
                    />
                    {errors.remote_port && <p className="glass-error-text">{errors.remote_port}</p>}
                  </div>
                </div>

                <div>
                  <label className="glass-label">远程主机</label>
                  {servers.length === 0 ? (
                    <input
                      type="text"
                      value={newMapping.remote_host || ''}
                      onChange={(e) => {
                        setNewMapping(prev => ({ ...prev, remote_host: e.target.value }));
                        setErrors(prev => ({ ...prev, remote_host: '' }));
                      }}
                      className={`glass-input ${errors.remote_host ? 'border-red-400/50' : ''}`}
                      placeholder="例如: 192.168.1.100"
                    />
                  ) : (
                    <div className="space-y-1.5 max-h-32 overflow-y-auto p-2 rounded-lg border border-white/10 bg-white/5">
                      {servers.map(server => (
                        <label
                          key={server.id}
                          className={`flex items-center gap-2.5 p-2 rounded-lg cursor-pointer transition-all ${
                            newMapping.remote_host === server.host
                              ? 'bg-info-light border border-info-border'
                              : 'hover:bg-white/5 border border-transparent'
                          }`}
                        >
                          <div className="relative">
                            <input
                              type="radio"
                              name="remote_host"
                              value={server.host}
                              checked={newMapping.remote_host === server.host}
                              onChange={(e) => {
                                setNewMapping(prev => ({ ...prev, remote_host: e.target.value }));
                                setErrors(prev => ({ ...prev, remote_host: '' }));
                              }}
                              className="w-4 h-4 rounded-full border-2 border-white/30 bg-white/10 checked:bg-info checked:border-info appearance-none cursor-pointer transition-colors"
                            />
                            {newMapping.remote_host === server.host && (
                              <div className="absolute top-1 left-1 w-2 h-2 rounded-full bg-info pointer-events-none" />
                            )}
                          </div>
                          <div className="flex-1">
                            <span className="text-primary text-sm font-medium">{server.name}</span>
                            <span className="text-quaternary text-xs ml-1.5">({server.host})</span>
                            {server.server_type === 'internal' || (server.server_type as unknown as number) === 1 ? (
                              <span className="ml-1.5 text-2xs px-1.5 py-0.5 rounded bg-warning-light text-warning-text border border-warning-border">内网</span>
                            ) : (
                              <span className="ml-1.5 text-2xs px-1.5 py-0.5 rounded bg-success-light text-success-text border border-success-border">外网</span>
                            )}
                          </div>
                        </label>
                      ))}
                      {/* 自定义主机选项 */}
                      <label
                        className={`flex items-center gap-2.5 p-2 rounded-lg cursor-pointer transition-all ${
                          newMapping.remote_host && !servers.find(s => s.host === newMapping.remote_host)
                            ? 'bg-info-light border border-info-border'
                            : 'hover:bg-white/5 border border-transparent'
                        }`}
                      >
                        <div className="relative">
                          <input
                            type="radio"
                            name="remote_host"
                            checked={Boolean(newMapping.remote_host === '' || (newMapping.remote_host && !servers.find(s => s.host === newMapping.remote_host)))}
                            onChange={() => {
                              setNewMapping(prev => ({ ...prev, remote_host: '' }));
                              setErrors(prev => ({ ...prev, remote_host: '' }));
                            }}
                            className="w-4 h-4 rounded-full border-2 border-white/30 bg-white/10 checked:bg-info checked:border-info appearance-none cursor-pointer transition-colors"
                          />
                          {(newMapping.remote_host === '' || (newMapping.remote_host && !servers.find(s => s.host === newMapping.remote_host))) && (
                            <div className="absolute top-1 left-1 w-2 h-2 rounded-full bg-info pointer-events-none" />
                          )}
                        </div>
                        <div className="flex-1">
                          <span className="text-secondary text-sm">自定义主机</span>
                        </div>
                      </label>
                      {(newMapping.remote_host === '' || (newMapping.remote_host && !servers.find(s => s.host === newMapping.remote_host))) && (
                        <input
                          type="text"
                          value={newMapping.remote_host || ''}
                          onChange={(e) => {
                            setNewMapping(prev => ({ ...prev, remote_host: e.target.value }));
                            setErrors(prev => ({ ...prev, remote_host: '' }));
                          }}
                          className="glass-input ml-6 mt-1"
                          placeholder="输入自定义 IP 或主机名"
                        />
                      )}
                    </div>
                  )}
                  {errors.remote_host && <p className="glass-error-text">{errors.remote_host}</p>}
                </div>
              </div>

              {/* Portal Server */}
              <div>
                <label className="glass-label">
                  Portal 服务器地址（可选）
                </label>
                <input
                  type="text"
                  value={newMapping.portal_server || ''}
                  onChange={(e) => {
                    setNewMapping(prev => ({ ...prev, portal_server: e.target.value }));
                  }}
                  className="glass-input"
                  placeholder="例如: gateway.example.com:8443 或 192.168.1.1:18888"
                />
                <p className="text-quaternary text-xs mt-1">
                  如果不填写，将自动使用中转节点的第一个外网服务器地址
                </p>
              </div>

              {/* Via Hops Selection */}
              <div className="p-3 rounded-lg border bg-brand-secondary/10 border border-brand-secondary/30">
                <div className="flex items-center justify-between mb-2">
                  <label className="glass-label" style={{ color: 'var(--color-brand-secondary)' }}>
                    中转节点（可选）
                  </label>
                </div>
                <p className="text-tertiary text-xs mb-3">选择跳板机以优化转发路径</p>

                {servers.filter(s => s.server_type === 'external' || (s.server_type as unknown as number) === 0).length === 0 ? (
                  <div className="text-center py-4 text-quaternary">
                    <div className="w-10 h-10 mx-auto mb-2 rounded-lg bg-white/5 flex items-center justify-center">
                      <svg width="20" height="20" className="text-quaternary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                      </svg>
                    </div>
                    <p className="text-sm">无可用的中转节点</p>
                  </div>
                ) : (
                  <div className="space-y-1.5 max-h-32 overflow-y-auto">
                    {servers
                      .filter(s => s.server_type === 'external' || (s.server_type as unknown as number) === 0)
                      .map(server => (
                        <label
                          key={server.id}
                          className={`flex items-center gap-2.5 p-2 rounded-lg cursor-pointer transition-all ${
                            (newMapping.via || []).includes(server.name)
                              ? 'bg-info-light border border-info-border'
                              : 'hover:bg-white/5 border border-transparent'
                          }`}
                        >
                          <div className="relative">
                            <input
                              type="checkbox"
                              checked={(newMapping.via || []).includes(server.name)}
                              onChange={() => toggleViaHop(server.name)}
                              className="w-4 h-4 rounded border-2 border-white/30 bg-white/10 checked:bg-info checked:border-info appearance-none cursor-pointer transition-colors"
                            />
                            {(newMapping.via || []).includes(server.name) && (
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
                )}
              </div>

              </div>

              {/* Fixed Footer */}
              <div className="glass-modal-footer glass-button-group-right">
                <button
                  type="button"
                  onClick={handleCloseModal}
                  className="glass-button"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="glass-button glass-button-primary"
                >
                  {submitting ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      {editingMapping ? '保存中...' : '创建中...'}
                    </>
                  ) : (
                    editingMapping ? '保存修改' : '创建映射'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
