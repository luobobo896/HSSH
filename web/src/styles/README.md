# GMSSH 样式系统快速参考

## 🎨 设计令牌

### 颜色
```css
/* 玻璃态背景 */
var(--glass-bg-base)        /* rgba(255, 255, 255, 0.06) */
var(--glass-bg-hover)       /* rgba(255, 255, 255, 0.08) */
var(--glass-bg-active)      /* rgba(255, 255, 255, 0.1) */

/* 玻璃态边框 */
var(--glass-border-default) /* rgba(255, 255, 255, 0.1) */
var(--glass-border-hover)   /* rgba(255, 255, 255, 0.15) */
var(--glass-border-focus)   /* rgba(34, 211, 238, 0.5) */

/* 文字颜色 */
var(--text-primary)         /* rgba(255, 255, 255, 0.95) */
var(--text-secondary)       /* rgba(255, 255, 255, 0.7) */
var(--text-tertiary)        /* rgba(255, 255, 255, 0.5) */
var(--text-quaternary)      /* rgba(255, 255, 255, 0.35) */

/* 语义化颜色 */
var(--color-success)        /* #4ade80 */
var(--color-warning)        /* #facc15 */
var(--color-error)          /* #f87171 */
var(--color-info)           /* #60a5fa */
```

### 字体大小
```css
var(--text-xs)              /* 12px */
var(--text-sm)              /* 13px */
var(--text-base)            /* 14px */
var(--text-md)              /* 15px */
var(--text-lg)              /* 16px */
var(--text-xl)              /* 18px */
var(--text-2xl)             /* 20px */
var(--text-3xl)             /* 24px */
```

### 间距
```css
var(--space-1)              /* 4px */
var(--space-2)              /* 8px */
var(--space-3)              /* 12px */
var(--space-4)              /* 16px */
var(--space-5)              /* 20px */
var(--space-6)              /* 24px */
var(--space-8)              /* 32px */
```

### 圆角
```css
var(--radius-sm)            /* 4px */
var(--radius-md)            /* 6px */
var(--radius-lg)            /* 8px */
var(--radius-xl)            /* 12px */
var(--radius-2xl)           /* 16px */
```

---

## 🧩 组件类

### 按钮
```tsx
// 基础按钮
<button className="glass-button">默认</button>

// 变体
<button className="glass-button glass-button-primary">主要</button>
<button className="glass-button glass-button-secondary">次要</button>
<button className="glass-button glass-button-success">成功</button>
<button className="glass-button glass-button-danger">危险</button>
<button className="glass-button glass-button-ghost">幽灵</button>

// 尺寸
<button className="glass-button glass-button-sm">小</button>
<button className="glass-button">默认</button>
<button className="glass-button glass-button-lg">大</button>

// 图标按钮
<button className="glass-button-icon">+</button>
<button className="glass-button-icon glass-button-icon-sm">+</button>

// 按钮组
<div className="glass-button-group">       {/* 等分宽度 */}
  <button className="glass-button">操作一</button>
  <button className="glass-button">操作二</button>
</div>

<div className="glass-button-group-right"> {/* 右对齐，用于弹窗底部 */}
  <button className="glass-button">取消</button>
  <button className="glass-button glass-button-primary">确认</button>
</div>
```

### 卡片
```tsx
// 基础卡片
<div className="glass-card">内容</div>

// 可交互卡片
<div className="glass-card glass-card-interactive">点击我</div>

// 扁平卡片
<div className="glass-card glass-card-flat">无边框阴影</div>
```

### 表单
```tsx
// 标签
<label className="glass-label">标签</label>
<label className="glass-label glass-label-required">必填标签</label>

// 输入框
<input className="glass-input" placeholder="提示文字" />
<input className="glass-input error" /> {/* 错误状态 */}

// 文本域
<textarea className="glass-input glass-textarea" />

// 选择器
<select className="glass-select">
  <option>选项</option>
</select>

// 帮助文本
<p className="glass-help-text">帮助说明</p>

// 错误消息
<p className="glass-error-text">错误信息</p>
```

### 导航
```tsx
<button className="glass-nav-item">
  <svg className="w-4 h-4" /> 导航项
</button>

<button className="glass-nav-item active">
  <svg className="w-4 h-4" /> 当前项
</button>
```

### 徽章
```tsx
// 语义化徽章
<span className="glass-badge glass-badge-success">成功</span>
<span className="glass-badge glass-badge-warning">警告</span>
<span className="glass-badge glass-badge-error">错误</span>
<span className="glass-badge glass-badge-info">信息</span>
<span className="glass-badge glass-badge-neutral">中性</span>

// 颜色别名（与语义化徽章等价）
<span className="glass-badge glass-badge-green">成功</span>   {/* = glass-badge-success */}
<span className="glass-badge glass-badge-yellow">警告</span>  {/* = glass-badge-warning */}
<span className="glass-badge glass-badge-blue">信息</span>   {/* = glass-badge-info */}
```

### 模态框
```tsx
// 标准弹窗结构
<div className="glass-modal-overlay">
  <div className="glass-modal glass-modal-lg animate-scale-in">
    <div className="glass-modal-header">
      <h2 className="glass-modal-title">标题</h2>
      <button className="glass-modal-close">×</button>
    </div>
    <div className="glass-modal-body">可滚动内容区域</div>
    <div className="glass-modal-footer glass-button-group-right">
      <button className="glass-button">取消</button>
      <button className="glass-button glass-button-primary">确认</button>
    </div>
  </div>
</div>

// 弹窗尺寸变体
.glass-modal-sm   /* 360px 宽, 60vh 最大高度 */
.glass-modal-md   /* 480px 宽, 70vh 最大高度 (默认) */
.glass-modal-lg   /* 640px 宽, 80vh 最大高度 */
.glass-modal-xl   /* 800px 宽, 85vh 最大高度 */
.glass-modal-full /* 90vw 宽, 90vh 最大高度 */
```

### 选项卡片
```tsx
<button className="glass-option-card">
  <span className="glass-option-card-icon">🌐</span>
  <span className="glass-option-card-title">标题</span>
  <span className="glass-option-card-description">描述</span>
</button>

<button className="glass-option-card selected">
  {/* 选中状态 */}
</button>
```

### 空状态
```tsx
<div className="glass-empty">
  <div className="glass-empty-icon">
    <svg />
  </div>
  <p className="glass-empty-title">暂无数据</p>
  <p className="glass-empty-description">添加一些内容吧</p>
</div>
```

### 加载状态
```tsx
<div className="glass-loading">
  <div className="glass-spinner" />
  <p className="glass-loading-text">加载中...</p>
</div>

<div className="glass-spinner glass-spinner-sm" />   {/* 小 */}
<div className="glass-spinner" />                     {/* 默认 */}
<div className="glass-spinner glass-spinner-lg" />   {/* 大 */}
```

### 进度条
```tsx
<div className="glass-progress">
  <div className="glass-progress-bar glass-progress-bar-info" style={{ width: '50%' }} />
</div>

<div className="glass-progress-bar glass-progress-bar-success" />  {/* 成功 */}
<div className="glass-progress-bar glass-progress-bar-error" />    {/* 错误 */}
```

---

## 🎯 Tailwind 扩展类

### 颜色
```tsx
// 玻璃态
<div className="bg-glass border-glass" />

// 语义化
<div className="bg-success-light border-success-border text-success-text" />
<div className="bg-warning-light border-warning-border text-warning-text" />
<div className="bg-error-light border-error-border text-error-text" />

// 文字
<p className="text-primary" />
<p className="text-secondary" />
<p className="text-tertiary" />
```

### 动画
```tsx
<div className="animate-fade-in-up" />
<div className="animate-fade-in" />
<div className="animate-scale-in" />
<div className="animate-slide-in-right" />
```

### 网格
```tsx
// 卡片网格
<div className="grid-cards">
  <div className="glass-card">...</div>
  <div className="glass-card">...</div>
</div>
```

---

## ❌ 避免使用

```tsx
/* 避免使用任意值 */
className="text-[13px]"       /* ❌ */
className="w-[400px]"         /* ❌ */
className="m-[15px]"          /* ❌ */

/* 避免直接写颜色值 */
className="text-white/50"     /* ❌ */
className="bg-blue-500/10"    /* ❌ */
className="border-red-400"    /* ❌ */

/* 避免混合不同间距 */
className="p-3.5"             /* ❌ */
className="gap-[10px]"        /* ❌ */

/* 避免使用内联样式 */
style={{ fontSize: '13px' }}  /* ❌ */
```

---

## ✅ 推荐使用

```tsx
/* 使用设计令牌 */
className="text-sm"           /* ✅ 13px */
className="w-128"             /* ✅ 512px */
className="m-4"               /* ✅ 16px */

/* 使用语义化颜色 */
className="text-tertiary"     /* ✅ */
className="bg-info-light"     /* ✅ */
className="border-error-border" /* ✅ */

/* 使用标准间距 */
className="p-3"               /* ✅ 12px */
className="gap-3"             /* ✅ 12px */

/* 使用组件类 */
className="glass-input"       /* ✅ */
className="glass-label"       /* ✅ */
className="glass-button-primary" /* ✅ */
```

---

## 🔧 开发提示

1. **始终优先使用组件类**，如 `glass-button` 而非原始 Tailwind 类
2. **使用 CSS 变量** 而非硬编码值
3. **保持 4px/8px 间距基准**
4. **检查可访问性** - 确保对比度符合 WCAG AA 标准
5. **避免混合使用** 任意值和 Tailwind 标准类

---

## 📚 相关文件

- `design-tokens.css` - 设计令牌定义
- `components.css` - 组件样式定义
- `../index.css` - 全局样式
- `../../tailwind.config.js` - Tailwind 配置
