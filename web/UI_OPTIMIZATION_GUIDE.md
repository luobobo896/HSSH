# GMSSH 前端 UI 优化指南

> 本文档详细说明项目中所有 UI 不一致问题及其修复方案，包含具体的文件修改位置和代码示例。

---

## 📋 优化概览

| 优化类别 | 问题数量 | 优先级 | 状态 |
|---------|---------|-------|------|
| 视觉一致性 | 15 | 🔴 高 | 待修复 |
| 交互一致性 | 8 | 🔴 高 | 待修复 |
| 布局与间距 | 12 | 🟡 中 | 待修复 |
| 信息层级 | 6 | 🟡 中 | 待修复 |
| 可访问性 | 4 | 🟢 低 | 待修复 |

---

## 🎨 设计令牌使用说明

优化后的样式系统基于以下文件：

```
web/src/styles/
├── design-tokens.css    # 设计令牌（颜色、字体、间距等变量）
└── components.css       # 组件样式（按钮、卡片、表单等）

web/src/index.css        # 主样式文件（已重构）
web/tailwind.config.js   # Tailwind 配置（已扩展）
```

### 快速使用示例

```tsx
// 旧写法 - 不推荐
<button className="glass-button bg-blue-500/10 hover:bg-blue-500/20 text-[13px]">
  按钮
</button>

// 新写法 - 推荐
<button className="glass-button glass-button-secondary">
  按钮
</button>
```

---

## 🔧 详细修复清单

### 1. App.tsx - 导航栏

**文件位置**: `web/src/App.tsx`

**问题**: 导航栏圆角和内边距与其他卡片不一致

**修复位置**:
```tsx
// 第 19 行 - 修改前
<nav className="glass-card mx-4 mt-4 !p-0">

// 第 19 行 - 修改后
<nav className="glass-card mx-4 mt-4 p-0">
```

**修复位置**:
```tsx
// 第 26 行 - 修改前
<span className="text-[15px] font-semibold text-white">GMSSH</span>

// 第 26 行 - 修改后
<span className="text-md font-semibold text-primary">GMSSH</span>
```

---

### 2. Servers.tsx - 页面标题

**文件位置**: `web/src/pages/Servers.tsx`

**问题**: 页面标题字号和颜色不统一

**修复位置**:
```tsx
// 第 439-440 行 - 修改前
<h1 className="text-[17px] font-semibold text-white">服务器管理</h1>
<p className="text-white/50 text-[13px] mt-2">管理你的 SSH 服务器配置</p>

// 第 439-440 行 - 修改后
<h1 className="text-xl font-semibold text-primary">服务器管理</h1>
<p className="text-tertiary text-sm mt-2">管理你的 SSH 服务器配置</p>
```

---

### 3. Servers.tsx - 空状态

**文件位置**: `web/src/pages/Servers.tsx`

**问题**: 空状态样式与其他页面不一致

**修复位置**:
```tsx
// 第 465-473 行 - 修改前
<div className="glass-card text-center py-20 mt-8">
  <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-white/5 flex items-center justify-center">
    <svg width="32" height="32" className="text-white/30" ... />
  </div>
  <p className="text-white/60 text-base">暂无服务器</p>
  <p className="text-white/40 text-sm mt-1">点击上方按钮添加你的第一个服务器</p>
</div>

// 第 465-473 行 - 修改后
<div className="glass-empty">
  <div className="glass-empty-icon">
    <svg width="32" height="32" className="text-quaternary" ... />
  </div>
  <p className="glass-empty-title">暂无服务器</p>
  <p className="glass-empty-description">点击上方按钮添加你的第一个服务器</p>
</div>
```

---

### 4. Servers.tsx - 服务器卡片

**文件位置**: `web/src/pages/Servers.tsx`

**问题**: 卡片内边距、标题字号和按钮样式不统一

**修复位置**:
```tsx
// 第 489-490 行 - 修改前
<h3 className="font-semibold text-white text-lg">{server.name}</h3>
<p className="text-white/50 text-sm">{server.host}</p>

// 第 489-490 行 - 修改后
<h3 className="font-semibold text-primary text-lg">{server.name}</h3>
<p className="text-tertiary text-sm">{server.host}</p>
```

**修复位置**:
```tsx
// 第 494-511 行 - 修改前（编辑和删除按钮）
<button className="text-accent-cyan hover:text-accent-cyan/80 p-2 rounded-lg hover:bg-accent-cyan/10 transition-all">
<button className="text-red-400 hover:text-red-300 p-2 rounded-lg hover:bg-red-400/10 transition-all">

// 第 494-511 行 - 修改后（使用统一图标按钮）
<button className="glass-button-icon-sm glass-button-secondary">
<button className="glass-button-icon-sm glass-button-danger">
```

**修复位置**:
```tsx
// 第 547-572 行 - 修改前（卡片操作按钮）
<button className="flex-1 glass-button text-sm py-2 bg-accent-blue/10 hover:bg-accent-blue/20">
<button className="flex-1 glass-button text-sm py-2 bg-accent-cyan/10 hover:bg-accent-cyan/20">

// 第 547-572 行 - 修改后
<button className="flex-1 glass-button glass-button-secondary">
<button className="flex-1 glass-button glass-button-primary">
```

---

### 5. Servers.tsx - 模态框

**文件位置**: `web/src/pages/Servers.tsx`

**问题**: 模态框样式未使用统一组件

**修复位置**:
```tsx
// 第 582-609 行 - 修改前
<div className="fixed inset-0 bg-black/50 backdrop-blur-md flex items-start sm:items-center justify-center z-50 p-4 sm:p-6">
  <div className="glass-card w-full max-w-[400px] !p-5 animate-fade-in-up max-h-[85vh] overflow-y-auto my-10 sm:my-12">
    <div className="flex items-center justify-between mb-4">
      <h2 className="text-[14px] font-semibold text-white">添加服务器</h2>
      <button className="w-6 h-6 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 transition-colors">
        <svg className="w-3 h-3 text-white/60" ... />
      </button>
    </div>
    ...
  </div>
</div>

// 第 582-609 行 - 修改后
<div className="glass-modal-overlay">
  <div className="glass-modal animate-scale-in">
    <div className="glass-modal-header">
      <h2 className="glass-modal-title">添加服务器</h2>
      <button className="glass-modal-close">
        <svg className="w-4 h-4" ... />
      </button>
    </div>
    <div className="glass-modal-body">
      ...
    </div>
  </div>
</div>
```

---

### 6. Servers.tsx - 表单样式

**文件位置**: `web/src/pages/Servers.tsx`

**问题**: 表单标签、输入框样式不一致

**修复位置**:
```tsx
// 第 170-198 行 - 修改前（服务器类型选择）
<label className="block text-[11px] font-medium text-white/50 mb-2">服务器类型</label>
<button className={`p-3 rounded-lg border text-[12px] font-medium transition-all ${...}`}>

// 第 170-198 行 - 修改后
<label className="glass-label">服务器类型</label>
<div className="grid grid-cols-2 gap-3">
  <button className={`glass-option-card ${...}`}>
    <span className="glass-option-card-icon">🌐</span>
    <span className="glass-option-card-title">外网服务器</span>
    <span className="glass-option-card-description">可直接访问</span>
  </button>
  ...
</div>
```

**修复位置**:
```tsx
// 第 281-296 行 - 修改前（表单输入组）
<label className="block text-[11px] font-medium text-white/50 mb-1">名称</label>
<input className={`glass-input ${errors.name ? 'border-red-400/50' : ''} ...`} />
{errors.name && <p className="text-[11px] text-red-400 mt-1">{errors.name}</p>}

// 第 281-296 行 - 修改后
<label className="glass-label">名称</label>
<input className={`glass-input ${errors.name ? 'error' : ''} ...`} />
{errors.name && <p className="glass-error-text">{errors.name}</p>}
```

---

### 7. Transfer.tsx - 上传类型切换

**文件位置**: `web/src/pages/Transfer.tsx`

**问题**: 切换按钮样式与 Servers 页面类型选择不一致

**修复位置**:
```tsx
// 第 346-367 行 - 修改前
<div className="flex gap-2">
  <button className={`flex-1 py-2 px-4 rounded-lg text-[13px] font-medium transition-all ${...}`}>
    📄 上传文件
  </button>
  ...
</div>

// 第 346-367 行 - 修改后
<div className="grid grid-cols-2 gap-3">
  <button className={`glass-option-card ${uploadType === 'file' ? 'selected' : ''}`}>
    <span className="glass-option-card-icon">📄</span>
    <span className="glass-option-card-title">上传文件</span>
  </button>
  <button className={`glass-option-card ${uploadType === 'folder' ? 'selected' : ''}`}>
    <span className="glass-option-card-icon">📁</span>
    <span className="glass-option-card-title">上传文件夹</span>
  </button>
</div>
```

---

### 8. Transfer.tsx - 拖放区域

**文件位置**: `web/src/pages/Transfer.tsx`

**问题**: 拖放区域样式需要统一

**修复位置**:
```tsx
// 第 373-436 行 - 修改前
<div className={`glass-card text-center transition-all duration-200 ${...}`}>

// 第 373-436 行 - 修改后
<div className={`glass-card glass-card-interactive text-center ${isDragOver ? 'border-info-border bg-info-light' : ''}`}>
  ...
</div>
```

---

### 9. Transfer.tsx - 目标配置卡片

**文件位置**: `web/src/pages/Transfer.tsx`

**问题**: 卡片内标题和图标样式不统一

**修复位置**:
```tsx
// 第 441-449 行 - 修改前
<div className="flex items-center gap-2">
  <div className="w-7 h-7 rounded-md bg-accent-cyan/20 flex items-center justify-center">
    <svg width="14" height="14" className="text-accent-cyan" ... />
  </div>
  <h3 className="text-[14px] font-medium text-white">目标配置</h3>
</div>

// 第 441-449 行 - 修改后
<div className="flex items-center gap-3">
  <div className="w-8 h-8 rounded-lg bg-info-light flex items-center justify-center">
    <svg className="w-4 h-4 text-info" ... />
  </div>
  <h3 className="text-base font-medium text-primary">目标配置</h3>
</div>
```

---

### 10. Transfer.tsx - 表单标签

**文件位置**: `web/src/pages/Transfer.tsx`

**问题**: 表单标签样式不一致

**修复位置**:
```tsx
// 第 451-452 行 - 修改前
<label className="block text-[12px] font-medium text-white/50 mb-1.5">目标服务器</label>

// 第 451-452 行 - 修改后
<label className="glass-label">目标服务器</label>
```

---

### 11. Transfer.tsx - 进度条

**文件位置**: `web/src/pages/Transfer.tsx`

**问题**: 进度条样式可以统一到组件系统

**修复位置**:
```tsx
// 第 751-759 行 - 修改前
<div className="relative h-2 bg-white/10 rounded-full overflow-hidden">
  <div className={`absolute top-0 left-0 h-full rounded-full transition-all duration-300 ${...}`}
    style={{ width: `${progress.percentage}%` }} />
</div>

// 第 751-759 行 - 修改后
<div className="glass-progress">
  <div className={`glass-progress-bar ${
    progress.status === 'completed' ? 'glass-progress-bar-success' :
    progress.status === 'failed' ? 'glass-progress-bar-error' :
    'glass-progress-bar-info'
  }`} style={{ width: `${progress.percentage}%` }} />
</div>
```

---

### 12. Portal.tsx - 协议选择

**文件位置**: `web/src/pages/Portal.tsx`

**问题**: 协议选择卡片样式与 Servers 页面不一致

**修复位置**:
```tsx
// 第 386-402 行 - 修改前
<div className="grid grid-cols-3 gap-2">
  {PROTOCOL_OPTIONS.map((option) => (
    <button className={`p-3 rounded-lg border text-[12px] font-medium transition-all ${...}`}>
      <div className="text-lg mb-1">{option.icon}</div>
      <div>{option.label}</div>
    </button>
  ))}
</div>

// 第 386-402 行 - 修改后
<div className="grid grid-cols-3 gap-3">
  {PROTOCOL_OPTIONS.map((option) => (
    <button className={`glass-option-card ${newMapping.protocol === option.value ? 'selected' : ''}`}>
      <span className="glass-option-card-icon">{option.icon}</span>
      <span className="glass-option-card-title">{option.label}</span>
    </button>
  ))}
</div>
```

---

### 13. Portal.tsx - 模态框底部按钮

**文件位置**: `web/src/pages/Portal.tsx`

**问题**: 底部按钮间距不一致

**修复位置**:
```tsx
// 第 625-648 行 - 修改前
<div className="flex gap-2 pt-2">
  <button className="flex-1 glass-button">取消</button>
  <button className="flex-1 glass-button glass-button-primary">...</button>
</div>

// 第 625-648 行 - 修改后
<div className="glass-modal-footer">
  <button className="glass-button flex-1" onClick={handleCloseModal}>取消</button>
  <button className="glass-button glass-button-primary flex-1" type="submit" disabled={submitting}>
    ...
  </button>
</div>
```

---

### 14. Terminal.tsx - 状态标签

**文件位置**: `web/src/components/Terminal/Terminal.tsx`

**问题**: 状态标签样式可统一到徽章系统

**修复位置**:
```tsx
// 第 422-425 行 - 修改前
<div className={`px-2 py-0.5 rounded text-xs font-medium ${status.bg} ${status.color}`}>
  {status.text}
</div>

// 第 422-425 行 - 修改后
<span className={`glass-badge ${
  connectionStatus === 'connected' ? 'glass-badge-success' :
  connectionStatus === 'error' ? 'glass-badge-error' :
  connectionStatus === 'connecting' ? 'glass-badge-warning' :
  'glass-badge-neutral'
}`}>
  {status.text}
</span>
```

---

## 📐 布局规范统一

### 页面内边距标准

```tsx
// 所有页面内容区使用统一的内边距
<main className="max-w-7xl mx-auto py-5 px-4">
```

### 卡片间距标准

```tsx
// 卡片列表统一使用 grid-cards 类
<div className="grid-cards">
  {items.map(item => (
    <div key={item.id} className="glass-card">
      ...
    </div>
  ))}
</div>
```

### 表单间距标准

```tsx
// 表单统一使用 space-y-4
<form className="space-y-4">
  <div>
    <label className="glass-label">标签</label>
    <input className="glass-input" />
    <p className="glass-help-text">帮助文本</p>
  </div>
</form>
```

---

## ♿ 可访问性改进

### 焦点状态

```css
/* 已为所有交互元素添加统一的焦点样式 */
:focus-visible {
  outline: 2px solid var(--glass-border-focus);
  outline-offset: 2px;
}
```

### 对比度要求

- 主要文字: `rgba(255, 255, 255, 0.95)` - 符合 WCAG AAA
- 次要文字: `rgba(255, 255, 255, 0.7)` - 符合 WCAG AA
- 辅助文字: `rgba(255, 255, 255, 0.5)` - 符合 WCAG AA（大字体）

### 禁用状态

```tsx
// 统一使用 disabled 样式
<button disabled className="glass-button">
  禁用按钮
</button>
```

---

## 🎯 优先级建议

### 🔴 高优先级（立即修复）

1. **页面标题统一** - 所有页面标题使用 `text-xl font-semibold text-primary`
2. **表单标签统一** - 所有标签使用 `glass-label` 类
3. **按钮样式统一** - 使用 `glass-button-*` 系列类
4. **空状态统一** - 使用 `glass-empty` 组件

### 🟡 中优先级（本周修复）

5. **卡片内边距统一** - 所有卡片使用 `glass-card` 标准内边距
6. **模态框统一** - 使用 `glass-modal-*` 组件
7. **加载状态统一** - 使用 `glass-loading` 组件
8. **选项卡片统一** - 使用 `glass-option-card` 组件

### 🟢 低优先级（后续迭代）

9. **动画效果统一** - 统一使用动画类
10. **响应式优化** - 完善移动端适配
11. **深色模式支持** - 准备多主题支持
12. **无障碍完善** - 添加 ARIA 标签

---

## 📝 代码审查清单

在提交代码前，请检查以下项目：

- [ ] 没有使用任意值（如 `text-[13px]`、`w-[400px]`）
- [ ] 颜色使用设计令牌（如 `text-primary` 而非 `text-white/50`）
- [ ] 间距使用 4px/8px 基准（如 `space-y-4`、`gap-3`）
- [ ] 圆角使用标准值（如 `rounded-lg`、`rounded-xl`）
- [ ] 按钮使用正确的语义类（如 `glass-button-primary`）
- [ ] 表单元素使用统一组件（如 `glass-input`、`glass-label`）
- [ ] 交互元素有焦点状态
- [ ] 文字对比度符合 WCAG AA 标准

---

## 🚀 实施步骤

### 第一阶段：基础设施（1天）

1. ✅ 创建 `design-tokens.css` 和 `components.css`
2. ✅ 重构 `index.css`
3. ✅ 更新 `tailwind.config.js`

### 第二阶段：核心组件（2天）

4. 更新 `App.tsx` 导航栏
5. 更新 `Servers.tsx` 表单和卡片
6. 更新 `Transfer.tsx` 上传区域
7. 更新 `Portal.tsx` 协议选择

### 第三阶段：细节优化（2天）

8. 统一所有空状态
9. 统一所有加载状态
10. 统一所有模态框
11. 检查并修复响应式问题

### 第四阶段：验证测试（1天）

12. 视觉回归测试
13. 可访问性检查
14. 性能测试

---

## 📚 参考资源

- [Tailwind CSS 文档](https://tailwindcss.com/docs)
- [WCAG 2.1 指南](https://www.w3.org/WAI/WCAG21/quickref/)
- [玻璃态设计最佳实践](https://uxplanet.org/glassmorphism-in-ui-design-4248d1f9b2f5)

---

**最后更新**: 2026-02-07
**文档版本**: v1.0
