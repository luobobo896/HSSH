---
title: HSSH UI 设计系统规范
date: 2025-01-14
type: Guide
status: Approved
---

# HSSH UI 设计系统规范

## 1. 色彩体系

### 1.1 品牌色
```css
--color-brand-primary: #ff6b9d;      /* 粉红 - 主要强调 */
--color-brand-secondary: #c084fc;    /* 紫色 - 次要强调 */
--color-brand-tertiary: #60a5fa;     /* 蓝色 - 信息 */
--color-brand-quaternary: #22d3ee;   /* 青色 - 高亮 */
```

### 1.2 语义色
```css
/* 成功 - Success */
--color-success: #4ade80;
--color-success-light: rgba(74, 222, 128, 0.2);
--color-success-border: rgba(74, 222, 128, 0.3);
--color-success-text: #86efac;

/* 警告 - Warning */
--color-warning: #facc15;
--color-warning-light: rgba(250, 204, 21, 0.2);
--color-warning-border: rgba(250, 204, 21, 0.3);
--color-warning-text: #fde047;

/* 错误 - Error */
--color-error: #f87171;
--color-error-light: rgba(248, 113, 113, 0.2);
--color-error-border: rgba(248, 113, 113, 0.3);
--color-error-text: #fca5a5;

/* 信息 - Info */
--color-info: #60a5fa;
--color-info-light: rgba(96, 165, 250, 0.2);
--color-info-border: rgba(96, 165, 250, 0.3);
--color-info-text: #22d3ee;
```

### 1.3 辅助色别名
```css
--color-accent-blue: var(--color-brand-tertiary);
--color-accent-cyan: var(--color-brand-quaternary);
--color-accent-pink: var(--color-brand-primary);
--color-accent-purple: var(--color-brand-secondary);
```

## 2. 弹窗组件规范

### 2.1 尺寸规范
| 尺寸 | 宽度 | 最大高度 | 用途 |
|-----|------|---------|------|
| sm | 360px | 60vh | 确认对话框、简单提示 |
| md (默认) | 480px | 70vh | 表单编辑、设置 |
| lg | 640px | 80vh | 复杂表单、详情展示 |
| xl | 800px | 85vh | 大屏表格、复杂操作 |
| full | 90vw | 90vh | 全屏操作、终端 |

### 2.2 结构规范
```html
<!-- 标准弹窗结构 -->
<div class="glass-modal-overlay">
  <div class="glass-modal glass-modal-lg animate-scale-in">
    <div class="glass-modal-header">
      <h2 class="glass-modal-title">标题</h2>
      <button class="glass-modal-close">...</button>
    </div>
    <div class="glass-modal-body">
      <!-- 可滚动内容区域 -->
    </div>
    <div class="glass-modal-footer glass-button-group-right">
      <button class="glass-button">取消</button>
      <button class="glass-button glass-button-primary">确认</button>
    </div>
  </div>
</div>
```

### 2.3 样式代码
```css
/* 遮罩层 */
.glass-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal-backdrop);
  padding: var(--space-4);
}

/* 弹窗容器 */
.glass-modal {
  background: var(--glass-bg-modal);
  backdrop-filter: blur(var(--blur-lg));
  border: 1px solid var(--glass-border-default);
  border-radius: var(--radius-xl);
  width: 100%;
  max-width: 480px;
  max-height: 85vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  z-index: var(--z-modal);
}

/* 弹窗尺寸变体 */
.glass-modal-sm { max-width: 360px; max-height: 60vh; }
.glass-modal-lg { max-width: 640px; max-height: 80vh; }
.glass-modal-xl { max-width: 800px; max-height: 85vh; }
.glass-modal-full { max-width: 90vw; max-height: 90vh; }

/* 弹窗内容区 */
.glass-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--glass-border-default);
  flex-shrink: 0;
}

.glass-modal-body {
  padding: var(--space-5);
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.glass-modal-footer {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-5);
  border-top: 1px solid var(--glass-border-default);
  flex-shrink: 0;
}
```

## 3. 按钮组件规范

### 3.1 类型与状态
```css
/* 主要按钮 */
.glass-button-primary {
  background: linear-gradient(135deg, var(--color-brand-primary) 0%, var(--color-brand-secondary) 100%);
  border: none;
  color: white;
}

.glass-button-primary:hover:not(:disabled) {
  opacity: 0.92;
  box-shadow: 4px 0 12px rgba(255, 107, 157, 0.25), -4px 0 12px rgba(255, 107, 157, 0.25);
  transform: translateY(-1px);
}

.glass-button-primary:active:not(:disabled) {
  opacity: 0.85;
  transform: translateY(0) scale(0.98);
}

/* 次要按钮 */
.glass-button-secondary {
  background: var(--color-info-light);
  border-color: var(--color-info-border);
  color: var(--color-info-text);
}

/* 成功按钮 */
.glass-button-success {
  background: var(--color-success-light);
  border-color: var(--color-success-border);
  color: var(--color-success-text);
}

/* 危险按钮 */
.glass-button-danger {
  background: var(--color-error-light);
  border-color: var(--color-error-border);
  color: var(--color-error-text);
}

/* 幽灵按钮 */
.glass-button-ghost {
  background: transparent;
  border-color: transparent;
}
```

### 3.2 尺寸规范
| 尺寸 | 高度 | 水平内边距 | 字体大小 | 图标大小 |
|-----|------|-----------|---------|---------|
| sm | 28px | 12px | 12px | 14px |
| md (默认) | 36px | 16px | 13px | 16px |
| lg | 44px | 24px | 14px | 18px |

### 3.3 按钮组
```css
/* 标准按钮组 - 等分宽度 */
.glass-button-group {
  display: flex;
  gap: var(--space-3);
}

.glass-button-group .glass-button {
  flex: 1;
}

/* 右对齐按钮组 - 用于弹窗底部 */
.glass-button-group-right {
  display: flex;
  gap: var(--space-3);
  justify-content: flex-end;
}
```

## 4. 徽章组件规范

### 4.1 颜色变体
```css
.glass-badge-success { /* 绿色 */ }
.glass-badge-warning { /* 黄色 */ }
.glass-badge-error { /* 红色 */ }
.glass-badge-info { /* 蓝色 */ }
.glass-badge-neutral { /* 中性灰 */ }

/* 颜色别名 */
.glass-badge-blue { /* 映射到 info */ }
.glass-badge-green { /* 映射到 success */ }
.glass-badge-yellow { /* 映射到 warning */ }
```

## 5. 阴影与底色规范

### 5.1 阴影系统
```css
/* 玻璃态阴影 - 弱化处理 */
--shadow-glass: 0 4px 24px rgba(0, 0, 0, 0.12);
--shadow-glass-hover: 0 8px 32px rgba(0, 0, 0, 0.18);

/* 禁止在玻璃态元素上使用重阴影 */
/* 错误: shadow-xl, shadow-2xl */
/* 正确: 依靠 backdrop-filter 和 border 创造层次 */
```

### 5.2 背景底色
```css
--glass-bg-base: rgba(255, 255, 255, 0.06);
--glass-bg-hover: rgba(255, 255, 255, 0.08);
--glass-bg-active: rgba(255, 255, 255, 0.1);
--glass-bg-card: rgba(255, 255, 255, 0.06);
--glass-bg-modal: rgba(255, 255, 255, 0.08);
--glass-bg-input: rgba(255, 255, 255, 0.08);
--glass-bg-input-focus: rgba(255, 255, 255, 0.12);
```

## 6. 使用示例

### 6.1 标准弹窗
```tsx
<div className="glass-modal-overlay">
  <div className="glass-modal glass-modal-lg animate-scale-in">
    <div className="glass-modal-header">
      <h2 className="glass-modal-title">编辑配置</h2>
      <button className="glass-modal-close">×</button>
    </div>
    <div className="glass-modal-body">
      {/* 表单内容 */}
    </div>
    <div className="glass-modal-footer glass-button-group-right">
      <button className="glass-button">取消</button>
      <button className="glass-button glass-button-primary">保存</button>
    </div>
  </div>
</div>
```

### 6.2 状态徽章
```tsx
<span className="glass-badge glass-badge-success">运行中</span>
<span className="glass-badge glass-badge-warning">待机</span>
<span className="glass-badge glass-badge-blue">信息</span>
<span className="glass-badge glass-badge-green">成功</span>
```

### 6.3 按钮组
```tsx
<div className="glass-button-group">
  <button className="glass-button">操作一</button>
  <button className="glass-button">操作二</button>
  <button className="glass-button">操作三</button>
</div>
```
