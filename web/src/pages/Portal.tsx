import { useEffect, useState } from 'react';
import { getMappings, createMapping, deleteMapping, CreateMappingRequest } from '../api/portal';
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
      await createMapping(newMapping as CreateMappingRequest);
      setShowAddForm(false);
      setNewMapping({
        local_addr: ':8080',
        remote_port: 80,
        protocol: 'tcp',
      });
      setErrors({});
      await loadMappings();
    } catch (err) {
      console.error('Failed to create mapping:', err);
    } finally {
      setSubmitting(false);
    }
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

  const getProtocolLabel = (protocol: PortalProtocol) => {
    return PROTOCOL_OPTIONS.find(p => p.value === protocol)?.label || protocol;
  };

  const getProtocolIcon = (protocol: PortalProtocol) => {
    return PROTOCOL_OPTIONS.find(p => p.value === protocol)?.icon || '🔌';
  };

  const getProtocolColor = (protocol: PortalProtocol) => {
    switch (protocol) {
      case 'tcp':
        return 'bg-blue-400/20 text-blue-400 border-blue-400/30';
      case 'http':
        return 'bg-green-400/20 text-green-400 border-green-400/30';
      case 'websocket':
        return 'bg-purple-400/20 text-purple-400 border-purple-400/30';
      default:
        return 'bg-white/10 text-white/60 border-white/20';
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
    <div className="space-y-5 animate-fade-in-up">
      {/* Header */}
      <div>
        <div className="mb-5">
          <h1 className="text-[17px] font-semibold text-white">端口转发</h1>
          <p className="text-white/50 text-[13px] mt-2">管理本地到远程服务器的端口映射</p>
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
        <div className="glass-card p-12 text-center">
          <div className="w-12 h-12 mx-auto mb-4 rounded-full border-2 border-accent-cyan/30 border-t-accent-cyan animate-spin"></div>
          <p className="text-white/60">加载中...</p>
        </div>
      )}

      {/* Mappings Grid */}
      {!loading && (
        <>
          {mappings.length === 0 ? (
            <div className="glass-card text-center py-20">
              <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-white/5 flex items-center justify-center">
                <svg width="32" height="32" className="text-white/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <p className="text-white/60 text-base">暂无端口映射</p>
              <p className="text-white/40 text-sm mt-1">点击上方按钮添加第一个端口映射</p>
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
                        <h3 className="font-semibold text-white text-lg">{mapping.name}</h3>
                        <p className="text-white/50 text-sm">{mapping.local_addr}</p>
                      </div>
                    </div>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={() => handleDelete(mapping.id)}
                        className="text-red-400 hover:text-red-300 p-2 rounded-lg hover:bg-red-400/10 transition-all"
                        title="删除"
                      >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  </div>

                  {/* Mapping Details */}
                  <div className="space-y-2 mb-4">
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-white/50">协议</span>
                      <span className={`glass-badge ${getProtocolColor(mapping.protocol)}`}>
                        {getProtocolIcon(mapping.protocol)} {getProtocolLabel(mapping.protocol)}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-white/50">本地地址</span>
                      <span className="text-white font-mono">{mapping.local_addr}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-white/50">远程目标</span>
                      <span className="text-white font-mono">{mapping.remote_host}:{mapping.remote_port}</span>
                    </div>
                    {mapping.via && mapping.via.length > 0 && (
                      <div className="flex justify-between text-sm">
                        <span className="text-white/50">中转节点</span>
                        <span className="text-white/80">{mapping.via.join(' → ')}</span>
                      </div>
                    )}
                    <div className="flex justify-between text-sm items-center">
                      <span className="text-white/50">状态</span>
                      <span className={`glass-badge ${mapping.active ? 'glass-badge-green' : 'glass-badge-yellow'}`}>
                        {mapping.active ? '🟢 活跃' : '⏸️ 待机'}
                      </span>
                    </div>
                    {mapping.connection_count !== undefined && (
                      <div className="flex justify-between text-sm">
                        <span className="text-white/50">连接数</span>
                        <span className="text-white">{mapping.connection_count}</span>
                      </div>
                    )}
                  </div>

                  {/* Connection Path Visualization */}
                  {mapping.via && mapping.via.length > 0 && (
                    <div className="mt-3 p-2.5 bg-white/5 border border-white/10 rounded-lg">
                      <div className="text-[11px] text-white/50 mb-1.5">转发路径</div>
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="text-[12px] bg-accent-cyan/20 text-accent-cyan px-1.5 py-0.5 rounded">本地</span>
                        {mapping.via.map((hop, idx) => (
                          <span key={idx} className="flex items-center gap-1.5">
                            <span className="text-white/30">→</span>
                            <span className="text-[12px] bg-accent-purple/20 text-accent-purple px-1.5 py-0.5 rounded">{hop}</span>
                          </span>
                        ))}
                        <span className="text-white/30">→</span>
                        <span className="text-[12px] bg-green-400/20 text-green-400 px-1.5 py-0.5 rounded">{mapping.remote_host}</span>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {/* Add Mapping Modal */}
      {showAddForm && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-md flex items-start sm:items-center justify-center z-50 p-4 sm:p-6">
          <div className="glass-card w-full max-w-[500px] !p-5 animate-fade-in-up max-h-[85vh] overflow-y-auto my-10 sm:my-12">
            {/* Modal Header */}
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-[14px] font-semibold text-white">添加端口映射</h2>
              <button
                onClick={() => {
                  setShowAddForm(false);
                  setNewMapping({
                    local_addr: ':8080',
                    remote_port: 80,
                    protocol: 'tcp',
                  });
                  setErrors({});
                }}
                className="w-6 h-6 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 transition-colors"
              >
                <svg className="w-3 h-3 text-white/60" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {/* Protocol Selection */}
              <div>
                <label className="block text-[11px] font-medium text-white/50 mb-2">协议类型</label>
                <div className="grid grid-cols-3 gap-2">
                  {PROTOCOL_OPTIONS.map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      onClick={() => setNewMapping(prev => ({ ...prev, protocol: option.value }))}
                      className={`p-3 rounded-lg border text-[12px] font-medium transition-all ${
                        newMapping.protocol === option.value
                          ? getProtocolColor(option.value)
                          : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                      }`}
                    >
                      <div className="text-lg mb-1">{option.icon}</div>
                      <div>{option.label}</div>
                    </button>
                  ))}
                </div>
              </div>

              {/* Basic Info */}
              <div className="space-y-3">
                <div>
                  <label className="block text-[11px] font-medium text-white/50 mb-1">名称</label>
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
                  {errors.name && <p className="text-[11px] text-red-400 mt-1">{errors.name}</p>}
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-medium text-white/50 mb-1">本地地址</label>
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
                    {errors.local_addr && <p className="text-[11px] text-red-400 mt-1">{errors.local_addr}</p>}
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-white/50 mb-1">远程端口</label>
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
                    {errors.remote_port && <p className="text-[11px] text-red-400 mt-1">{errors.remote_port}</p>}
                  </div>
                </div>

                <div>
                  <label className="block text-[11px] font-medium text-white/50 mb-1">远程主机</label>
                  <input
                    type="text"
                    value={newMapping.remote_host || ''}
                    onChange={(e) => {
                      setNewMapping(prev => ({ ...prev, remote_host: e.target.value }));
                      setErrors(prev => ({ ...prev, remote_host: '' }));
                    }}
                    className={`glass-input ${errors.remote_host ? 'border-red-400/50' : ''}`}
                    placeholder="例如: 192.168.1.100 或 internal-db"
                  />
                  {errors.remote_host && <p className="text-[11px] text-red-400 mt-1">{errors.remote_host}</p>}
                </div>
              </div>

              {/* Via Hops Selection */}
              <div className="p-3 rounded-lg border bg-accent-purple/10 border-accent-purple/30">
                <div className="flex items-center justify-between mb-2">
                  <label className="block text-[11px] font-medium text-accent-purple/80">
                    中转节点（可选）
                  </label>
                </div>
                <p className="text-white/50 text-[11px] mb-3">选择跳板机以优化转发路径</p>

                {servers.filter(s => s.server_type === 'external' || (s.server_type as unknown as number) === 0).length === 0 ? (
                  <div className="text-center py-4 text-white/40">
                    <div className="w-10 h-10 mx-auto mb-2 rounded-lg bg-white/5 flex items-center justify-center">
                      <svg width="20" height="20" className="text-white/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                      </svg>
                    </div>
                    <p className="text-[12px]">无可用的中转节点</p>
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
                              ? 'bg-accent-cyan/10 border border-accent-cyan/30'
                              : 'hover:bg-white/5 border border-transparent'
                          }`}
                        >
                          <div className="relative">
                            <input
                              type="checkbox"
                              checked={(newMapping.via || []).includes(server.name)}
                              onChange={() => toggleViaHop(server.name)}
                              className="w-4 h-4 rounded border-2 border-white/30 bg-white/10 checked:bg-accent-cyan checked:border-accent-cyan appearance-none cursor-pointer transition-colors"
                            />
                            {(newMapping.via || []).includes(server.name) && (
                              <svg width="10" height="10" className="text-white absolute top-0.5 left-0.5 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                              </svg>
                            )}
                          </div>
                          <div className="flex-1">
                            <span className="text-white text-[13px] font-medium">{server.name}</span>
                            <span className="text-white/40 text-[12px] ml-1.5">({server.host})</span>
                          </div>
                        </label>
                      ))}
                  </div>
                )}
              </div>

              {/* Action Buttons */}
              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setShowAddForm(false);
                    setNewMapping({
                      local_addr: ':8080',
                      remote_port: 80,
                      protocol: 'tcp',
                    });
                    setErrors({});
                  }}
                  className="flex-1 glass-button"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="flex-1 glass-button glass-button-primary disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {submitting ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      创建中...
                    </>
                  ) : (
                    '创建映射'
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
