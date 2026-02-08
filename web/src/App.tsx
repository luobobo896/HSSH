import { useState } from 'react';
import { Servers } from './pages/Servers';
import { Transfer } from './pages/Transfer';
import { Portal } from './pages/Portal';

type Tab = 'servers' | 'transfer' | 'proxy';

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('servers');

  return (
    <div className="min-h-screen relative">
      {/* Background Orbs */}
      <div className="orb orb-1"></div>
      <div className="orb orb-2"></div>
      <div className="orb orb-3"></div>

      {/* Navigation */}
      <nav className="glass-card mx-4 mt-4 p-0">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex h-12 items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-brand-primary to-brand-secondary flex items-center justify-center text-white font-bold text-sm">
                ◆
              </div>
              <span className="text-md font-semibold text-primary">HSSH</span>
            </div>
            <div className="flex gap-1">
              <button
                onClick={() => setActiveTab('servers')}
                className={`glass-nav-item ${activeTab === 'servers' ? 'glass-nav-item-active' : ''}`}
              >
                <span className="flex items-center gap-1.5">
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                  </svg>
                  服务器
                </span>
              </button>
              <button
                onClick={() => setActiveTab('transfer')}
                className={`glass-nav-item ${activeTab === 'transfer' ? 'glass-nav-item-active' : ''}`}
              >
                <span className="flex items-center gap-1.5">
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                  </svg>
                  文件传输
                </span>
              </button>
              <button
                onClick={() => setActiveTab('proxy')}
                className={`glass-nav-item ${activeTab === 'proxy' ? 'glass-nav-item-active' : ''}`}
              >
                <span className="flex items-center gap-1.5">
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                  端口转发
                </span>
              </button>
            </div>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto py-5 px-4">
        {activeTab === 'servers' && <Servers onNavigateToTransfer={() => setActiveTab('transfer')} />}
        {activeTab === 'transfer' && <Transfer />}
        {activeTab === 'proxy' && <Portal />}
      </main>
    </div>
  );
}

export default App;
