import { useEffect, useState } from 'react';
import { useServerStore } from '../stores/serverStore';
import { Server } from '../types';
import { Terminal } from '../components/Terminal';

interface ServersProps {
  onNavigateToTransfer?: () => void;
}

export function Servers({ onNavigateToTransfer }: ServersProps) {
  const { servers, loading, fetchServers, addServer, updateServer, deleteServer, setPreselectedServer } = useServerStore();
  const [showAddForm, setShowAddForm] = useState(false);
  const [showEditForm, setShowEditForm] = useState(false);
  const [editingServer, setEditingServer] = useState<Server | null>(null);
  const [terminalServer, setTerminalServer] = useState<Server | null>(null);
  const [newServer, setNewServer] = useState<Partial<Server>>({
    port: 22,
    auth_type: 'key',
    key_path: '~/.ssh/id_rsa',
    server_type: 'external',
  });
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    fetchServers();
  }, [fetchServers]);

  const validateForm = (isEdit = false): boolean => {
    const newErrors: Record<string, string> = {};
    const serverData = isEdit ? editingServer : newServer;
    
    if (!serverData?.name?.trim()) {
      newErrors.name = '请输入服务器名称';
    }
    if (!serverData?.host?.trim()) {
      newErrors.host = '请输入主机地址';
    }
    if (!serverData?.user?.trim()) {
      newErrors.user = '请输入用户名';
    }
    
    // 内网服务器必须配置网关
    if (serverData?.server_type === 'internal' && !serverData?.gateway_id) {
      newErrors.gateway = '内网服务器必须选择网关';
    }

    // 验证网关不能是自己（通过 ID 比较）
    if (serverData?.server_type === 'internal' && serverData?.gateway_id === (isEdit ? editingServer?.id : undefined)) {
      newErrors.gateway = '网关不能是当前服务器';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!validateForm()) {
      return;
    }
    
    if (newServer.name && newServer.host && newServer.user) {
      await addServer(newServer as Omit<Server, 'id'>);
      setShowAddForm(false);
      setNewServer({ port: 22, auth_type: 'key', key_path: '~/.ssh/id_rsa', server_type: 'external' });
      setErrors({});
    }
  };

  const handleEdit = (server: Server) => {
    console.log('[DEBUG] handleEdit called with server:', JSON.stringify(server));
    // 将 server_type 和 auth_type 统一转换为字符串，避免数字和字符串混用
    const serverTypeNum = server.server_type as unknown as number;
    const authTypeNum = server.auth_type as unknown as number;
    const normalizedServer = {
      ...server,
      server_type: (serverTypeNum === 1 || server.server_type === 'internal') ? 'internal' : 'external',
      auth_type: (authTypeNum === 1 || server.auth_type === 'password') ? 'password' : 'key',
    };
    console.log('[DEBUG] normalizedServer:', JSON.stringify(normalizedServer));
    setEditingServer(normalizedServer as Server);
    setShowEditForm(true);
    setErrors({});
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm(true)) {
      return;
    }

    if (editingServer && editingServer.id) {
      const { id, ...updates } = editingServer;
      console.log('[DEBUG] handleUpdate - editingServer:', JSON.stringify(editingServer));
      console.log('[DEBUG] handleUpdate - updates:', JSON.stringify(updates));
      await updateServer(id, updates);
      setShowEditForm(false);
      setEditingServer(null);
      setErrors({});
    }
  };

  const handleServerTypeChange = (type: 'external' | 'internal', isEdit = false) => {
    if (isEdit) {
      setEditingServer(prev => prev ? {
        ...prev,
        server_type: type,
        // 切换类型时保留 gateway 配置，让用户自行决定是否清除
      } : null);
    } else {
      setNewServer(prev => ({
        ...prev,
        server_type: type,
      }));
    }
    // 清除相关错误
    setErrors(prev => ({ ...prev, gateway: '' }));
  };

  const getAuthIcon = (authType: string | number) => {
    const authTypeNum = authType as unknown as number;
    return (authType === 'key' || authTypeNum === 0) ? '🔑' : '🔒';
  };

  const getAuthLabel = (authType: string | number) => {
    const authTypeNum = authType as unknown as number;
    return (authType === 'key' || authTypeNum === 0) ? 'SSH 密钥' : '密码';
  };

  const getServerTypeIcon = (serverType: string | number) => {
    return (serverType === 'internal' || (serverType as unknown as number) === 1) ? '🔒' : '🌐';
  };

  const getServerTypeLabel = (serverType: string | number) => {
    return (serverType === 'internal' || (serverType as unknown as number) === 1) ? '内网' : '外网';
  };

  // 获取可用的网关服务器列表（外网服务器）
  // 后端返回 server_type 为数字: 0=external, 1=internal
  const getAvailableGateways = (excludeId?: string) => {
    return servers.filter(s => {
      const serverTypeNum = s.server_type as unknown as number;
      return (s.server_type === 'external' || serverTypeNum === 0) && s.id !== excludeId;
    });
  };

  // 通过 gateway_id 获取网关显示名称
  const getGatewayName = (gatewayId: string): string => {
    const gateway = servers.find(s => s.id === gatewayId);
    return gateway?.name || gatewayId.slice(0, 8) + '...';
  };

  // 渲染服务器表单
  const renderServerForm = (
    serverData: Partial<Server> | null,
    isEdit: boolean,
    onSubmit: (e: React.FormEvent) => void,
    onCancel: () => void
  ) => {
    const data = serverData || {};
    const serverId = isEdit ? editingServer?.id : undefined;
    const availableGateways = getAvailableGateways(serverId);

    return (
      <form onSubmit={onSubmit} className="space-y-4">
        {/* Server Type Selection */}
        <div>
          <label className="glass-label">服务器类型</label>
          <div className="grid grid-cols-2 gap-3">
            <button
              type="button"
              onClick={() => handleServerTypeChange('external', isEdit)}
              className={`glass-option-card ${data.server_type === 'external' ? 'selected' : ''}`}
            >
              <span className="glass-option-card-icon">🌐</span>
              <span className="glass-option-card-title">外网服务器</span>
              <span className="glass-option-card-description">可直接访问</span>
            </button>
            <button
              type="button"
              onClick={() => handleServerTypeChange('internal', isEdit)}
              className={`glass-option-card ${data.server_type === 'internal' ? 'selected' : ''}`}
            >
              <span className="glass-option-card-icon">🔒</span>
              <span className="glass-option-card-title">内网服务器</span>
              <span className="glass-option-card-description">需要网关中转</span>
            </button>
          </div>
        </div>

        {/* Gateway Selection - Available for all server types */}
        <div className={`p-3 rounded-lg border ${data.server_type === 'internal' ? 'bg-yellow-500/10 border-yellow-500/30' : 'bg-blue-500/10 border-blue-500/30'}`}>
          <div className="flex items-center justify-between mb-2">
            <label className={`glass-label ${data.server_type === 'internal' ? 'glass-label-required' : ''}`}>
              跳板机/网关
            </label>
            {data.gateway_id && (
              <button
                type="button"
                onClick={() => {
                  if (isEdit) {
                    setEditingServer(prev => prev ? { ...prev, gateway_id: undefined, gateway_name: undefined } : null);
                  } else {
                    setNewServer(prev => ({ ...prev, gateway_id: undefined, gateway_name: undefined }));
                  }
                }}
                className="text-xs text-quaternary hover:text-tertiary transition-colors"
              >
                清除
              </button>
            )}
          </div>
          {availableGateways.length === 0 ? (
            <div className="glass-error-text">
              ⚠️ 没有可用的外网服务器作为跳板机，请先添加一个外网服务器
            </div>
          ) : (
            <>
              <select
                value={data.gateway_id || ''}
                onChange={(e) => {
                  const selectedId = e.target.value;
                  const selectedGateway = availableGateways.find(g => g.id === selectedId);
                  console.log('[DEBUG] Gateway selected:', selectedId, 'isEdit:', isEdit);
                  if (isEdit) {
                    setEditingServer(prev => {
                      const updated = prev ? {
                        ...prev,
                        gateway_id: selectedId || undefined,
                        gateway_name: selectedGateway?.name
                      } : null;
                      console.log('[DEBUG] setEditingServer:', JSON.stringify(updated));
                      return updated;
                    });
                  } else {
                    setNewServer(prev => ({
                      ...prev,
                      gateway_id: selectedId || undefined,
                      gateway_name: selectedGateway?.name
                    }));
                  }
                  setErrors(prev => ({ ...prev, gateway: '' }));
                }}
                className={`glass-select ${errors.gateway ? 'error' : ''}`}
              >
                <option value="">
                  {data.server_type === 'internal' ? '选择网关服务器...' : '直接连接（不通过跳板机）...'}
                </option>
                {availableGateways.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.host}:{s.port})
                  </option>
                ))}
              </select>
              {errors.gateway && (
                <p className="glass-error-text">{errors.gateway}</p>
              )}
              <p className="text-xs text-quaternary mt-1.5">
                {data.server_type === 'internal'
                  ? '内网服务器必须通过外网服务器作为网关访问'
                  : '外网服务器也可以选择跳板机中转，改善连接质量'}
              </p>
            </>
          )}
        </div>

        {/* Basic Info */}
        <div className="space-y-3">
          <div>
            <label className="glass-label">名称</label>
            <input
              type="text"
              value={data.name || ''}
              onChange={(e) => {
                if (!isEdit) {
                  setNewServer(prev => ({ ...prev, name: e.target.value }));
                }
                setErrors(prev => ({ ...prev, name: '' }));
              }}
              disabled={isEdit}
              className={`glass-input ${errors.name ? 'error' : ''} ${isEdit ? 'opacity-60 cursor-not-allowed' : ''}`}
              placeholder="例如: gateway, db-server"
            />
            {isEdit && <p className="glass-help-text">服务器名称不可修改</p>}
            {errors.name && <p className="glass-error-text">{errors.name}</p>}
          </div>

          <div>
            <label className="glass-label">主机地址</label>
            <input
              type="text"
              value={data.host || ''}
              onChange={(e) => {
                if (isEdit) {
                  setEditingServer(prev => prev ? { ...prev, host: e.target.value } : null);
                } else {
                  setNewServer(prev => ({ ...prev, host: e.target.value }));
                }
                setErrors(prev => ({ ...prev, host: '' }));
              }}
              className={`glass-input ${errors.host ? 'error' : ''}`}
              placeholder="例如: 192.168.1.100 或 server.example.com"
            />
            {errors.host && <p className="glass-error-text">{errors.host}</p>}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="glass-label">端口</label>
              <input
                type="number"
                value={data.port || 22}
                onChange={(e) => {
                  if (isEdit) {
                    setEditingServer(prev => prev ? { ...prev, port: parseInt(e.target.value) } : null);
                  } else {
                    setNewServer(prev => ({ ...prev, port: parseInt(e.target.value) }));
                  }
                }}
                className="glass-input"
              />
            </div>
            <div>
              <label className="glass-label">用户名</label>
              <input
                type="text"
                value={data.user || ''}
                onChange={(e) => {
                  if (isEdit) {
                    setEditingServer(prev => prev ? { ...prev, user: e.target.value } : null);
                  } else {
                    setNewServer(prev => ({ ...prev, user: e.target.value }));
                  }
                  setErrors(prev => ({ ...prev, user: '' }));
                }}
                className={`glass-input ${errors.user ? 'error' : ''}`}
                placeholder="root"
              />
              {errors.user && <p className="glass-error-text">{errors.user}</p>}
            </div>
          </div>

          <div>
            <label className="glass-label">认证方式</label>
            <select
              value={data.auth_type}
              onChange={(e) => {
                if (isEdit) {
                  setEditingServer(prev => prev ? { ...prev, auth_type: e.target.value as 'key' | 'password' } : null);
                } else {
                  setNewServer(prev => ({ ...prev, auth_type: e.target.value as 'key' | 'password' }));
                }
              }}
              className="glass-select"
            >
              <option value="key">🔑 SSH 密钥</option>
              <option value="password">🔒 密码</option>
            </select>
          </div>

          {data.auth_type === 'key' && (
            <div>
              <label className="glass-label">密钥路径</label>
              <input
                type="text"
                value={data.key_path || ''}
                onChange={(e) => {
                  if (isEdit) {
                    setEditingServer(prev => prev ? { ...prev, key_path: e.target.value } : null);
                  } else {
                    setNewServer(prev => ({ ...prev, key_path: e.target.value }));
                  }
                }}
                className="glass-input"
                placeholder="~/.ssh/id_rsa"
              />
            </div>
          )}

          {data.auth_type === 'password' && (
            <div>
              <label className="glass-label">
                密码 {isEdit && data.password && <span className="text-tertiary">(已设置)</span>}
              </label>
              <input
                type="password"
                value={data.password || ''}
                onChange={(e) => {
                  if (isEdit) {
                    setEditingServer(prev => prev ? { ...prev, password: e.target.value } : null);
                  } else {
                    setNewServer(prev => ({ ...prev, password: e.target.value }));
                  }
                }}
                className="glass-input"
                placeholder='输入密码'
              />
            </div>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex gap-2 pt-2">
          <button
            type="button"
            onClick={onCancel}
            className="flex-1 glass-button"
          >
            取消
          </button>
          <button
            type="submit"
            disabled={data.server_type === 'internal' && availableGateways.length === 0}
            className="flex-1 glass-button glass-button-primary disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isEdit ? '保存修改' : '保存'}
          </button>
        </div>
      </form>
    );
  };

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Header */}
      <div>
        <div className="mb-5">
          <h1 className="text-xl font-semibold text-primary">服务器管理</h1>
          <p className="text-tertiary text-sm mt-2">管理你的 SSH 服务器配置</p>
        </div>
        <button
          onClick={() => setShowAddForm(true)}
          className="glass-button glass-button-primary mb-6"
        >
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
          添加服务器
        </button>
      </div>

      {/* Loading State */}
      {loading && (
        <div className="glass-card p-12 text-center">
          <div className="w-12 h-12 mx-auto mb-4 rounded-full border-2 border-info-border border-t-info animate-spin"></div>
          <p className="text-secondary">加载中...</p>
        </div>
      )}

      {/* Servers Grid */}
      {!loading && (
        <>
          {servers.length === 0 ? (
            <div className="glass-empty">
              <div className="glass-empty-icon">
                <svg width="32" height="32" className="text-quaternary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                </svg>
              </div>
              <p className="glass-empty-title">暂无服务器</p>
              <p className="glass-empty-description">点击上方按钮添加你的第一个服务器</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mt-8">
              {servers.map((server, index) => (
                <div
                  key={server.name}
                  className="glass-card p-5 group"
                  style={{ animationDelay: `${index * 0.1}s` }}
                >
                  {/* Card Header */}
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                      <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-brand-tertiary/30 to-brand-quaternary/30 flex items-center justify-center text-2xl">
                        {getServerTypeIcon(server.server_type)}
                      </div>
                      <div>
                        <h3 className="font-semibold text-primary text-lg">{server.name}</h3>
                        <p className="text-tertiary text-sm">{server.host}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={() => handleEdit(server)}
                        className="glass-button-icon-sm glass-button-secondary"
                        title="编辑"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                        </svg>
                      </button>
                      <button
                        onClick={() => deleteServer(server.id)}
                        className="glass-button-icon-sm glass-button-danger"
                        title="删除"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  </div>

                  {/* Server Details */}
                  <div className="space-y-2 mb-4">
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-tertiary">类型</span>
                      <div className="flex items-center gap-2">
                        <span className={`glass-badge ${(server.server_type === 'internal' || (server.server_type as unknown as number) === 1) ? 'glass-badge-yellow' : 'glass-badge-blue'}`}>
                          {getServerTypeIcon(server.server_type)} {getServerTypeLabel(server.server_type)}
                        </span>
                        {(server.server_type === 'internal' || (server.server_type as unknown as number) === 1) && server.gateway_id && (
                          <span className="text-secondary text-xs flex items-center gap-1">
                            → {getGatewayName(server.gateway_id)}
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-tertiary">端口</span>
                      <span className="text-primary font-mono">{server.port}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-tertiary">用户</span>
                      <span className="text-primary">{server.user}</span>
                    </div>
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-tertiary">认证</span>
                      <span className={`glass-badge ${server.auth_type === 'key' ? 'glass-badge-green' : 'glass-badge-yellow'}`}>
                        {getAuthIcon(server.auth_type)} {getAuthLabel(server.auth_type)}
                      </span>
                    </div>
                  </div>

                  {/* Action Buttons */}
                  <div className="flex gap-2">
                    <button 
                      onClick={() => setTerminalServer(server)}
                      className="flex-1 glass-button glass-button-secondary"
                      title="连接终端"
                      data-testid="connect-terminal-btn"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                      </svg>
                      连接终端
                    </button>
                    <button 
                      onClick={() => {
                        setPreselectedServer(server.id);
                        onNavigateToTransfer?.();
                      }}
                      className="flex-1 glass-button glass-button-primary"
                      title="传输文件到该服务器"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                      </svg>
                      传输文件
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {/* Add Server Modal */}
      {showAddForm && (
        <div className="glass-modal-overlay">
          <div className="glass-modal animate-scale-in">
            {/* Modal Header */}
            <div className="glass-modal-header">
              <h2 className="glass-modal-title">添加服务器</h2>
              <button
                onClick={() => setShowAddForm(false)}
                className="glass-modal-close"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="glass-modal-body">
              {renderServerForm(
                newServer,
                false,
                handleSubmit,
                () => {
                  setShowAddForm(false);
                  setNewServer({ port: 22, auth_type: 'key', key_path: '~/.ssh/id_rsa', server_type: 'external' });
                  setErrors({});
                }
              )}
            </div>
          </div>
        </div>
      )}

      {/* Edit Server Modal */}
      {showEditForm && editingServer && (
        <div className="glass-modal-overlay">
          <div className="glass-modal animate-scale-in">
            {/* Modal Header */}
            <div className="glass-modal-header">
              <h2 className="glass-modal-title">编辑服务器</h2>
              <button
                onClick={() => {
                  setShowEditForm(false);
                  setEditingServer(null);
                  setErrors({});
                }}
                className="glass-modal-close"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="glass-modal-body">
              {renderServerForm(
                editingServer,
                true,
                handleUpdate,
                () => {
                  setShowEditForm(false);
                  setEditingServer(null);
                  setErrors({});
                }
              )}
            </div>
          </div>
        </div>
      )}

      {/* Terminal Modal */}
      <Terminal
        server={terminalServer!}
        isOpen={!!terminalServer}
        onClose={() => setTerminalServer(null)}
        onError={(error) => console.error('[Terminal] Error:', error)}
      />
    </div>
  );
}
