/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      /* ============================================
         颜色系统扩展
         ============================================ */
      colors: {
        // 玻璃态颜色
        glass: {
          DEFAULT: 'rgba(255, 255, 255, 0.08)',
          base: 'rgba(255, 255, 255, 0.06)',
          hover: 'rgba(255, 255, 255, 0.08)',
          active: 'rgba(255, 255, 255, 0.1)',
          border: {
            DEFAULT: 'rgba(255, 255, 255, 0.1)',
            hover: 'rgba(255, 255, 255, 0.15)',
            focus: 'rgba(34, 211, 238, 0.5)',
          },
        },
        // 品牌色
        brand: {
          primary: '#ff6b9d',
          secondary: '#c084fc',
          tertiary: '#60a5fa',
          quaternary: '#22d3ee',
        },
        // 语义化颜色
        success: {
          DEFAULT: '#4ade80',
          light: 'rgba(74, 222, 128, 0.2)',
          border: 'rgba(74, 222, 128, 0.3)',
          text: '#86efac',
        },
        warning: {
          DEFAULT: '#facc15',
          light: 'rgba(250, 204, 21, 0.2)',
          border: 'rgba(250, 204, 21, 0.3)',
          text: '#fde047',
        },
        error: {
          DEFAULT: '#f87171',
          light: 'rgba(248, 113, 113, 0.2)',
          border: 'rgba(248, 113, 113, 0.3)',
          text: '#fca5a5',
        },
        info: {
          DEFAULT: '#60a5fa',
          light: 'rgba(96, 165, 250, 0.2)',
          border: 'rgba(96, 165, 250, 0.3)',
          text: '#22d3ee',
        },
        // 文字颜色
        text: {
          primary: 'rgba(255, 255, 255, 0.95)',
          secondary: 'rgba(255, 255, 255, 0.7)',
          tertiary: 'rgba(255, 255, 255, 0.5)',
          quaternary: 'rgba(255, 255, 255, 0.35)',
        },
      },

      /* ============================================
         字体系统扩展
         ============================================ */
      fontFamily: {
        primary: ['Outfit', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
        mono: ['Menlo', 'Monaco', 'Courier New', 'monospace'],
      },
      fontSize: {
        '2xs': '11px',
        xs: '12px',
        sm: '13px',
        base: '14px',
        md: '15px',
        lg: '16px',
        xl: '18px',
        '2xl': '20px',
        '3xl': '24px',
      },

      /* ============================================
         间距系统扩展 - 基于 4px/8px 基准
         ============================================ */
      spacing: {
        '18': '4.5rem',  // 72px
        '88': '22rem',   // 352px
        '128': '32rem',  // 512px
      },

      /* ============================================
         圆角系统扩展
         ============================================ */
      borderRadius: {
        '4xl': '2rem',   // 32px
      },

      /* ============================================
         阴影系统扩展
         ============================================ */
      boxShadow: {
        'glass': '0 4px 24px rgba(0, 0, 0, 0.12)',
        'glass-hover': '0 8px 32px rgba(0, 0, 0, 0.18)',
        'glass-lg': '0 16px 48px rgba(0, 0, 0, 0.25)',
        'inner-glass': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)',
      },

      /* ============================================
         模糊效果扩展
         ============================================ */
      backdropBlur: {
        'glass': '20px',
        'glass-lg': '40px',
      },

      /* ============================================
         过渡动画扩展
         ============================================ */
      transitionDuration: {
        '400': '400ms',
      },
      transitionTimingFunction: {
        'bounce-out': 'cubic-bezier(0.34, 1.56, 0.64, 1)',
      },

      /* ============================================
         关键帧动画扩展
         ============================================ */
      keyframes: {
        fadeInUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        slideInLeft: {
          '0%': { opacity: '0', transform: 'translateX(-20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        float: {
          '0%, 100%': { transform: 'translate(0, 0) scale(1)' },
          '25%': { transform: 'translate(50px, -50px) scale(1.1)' },
          '50%': { transform: 'translate(0, 50px) scale(0.95)' },
          '75%': { transform: 'translate(-50px, -25px) scale(1.05)' },
        },
        pulse: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.5' },
        },
        spin: {
          '0%': { transform: 'rotate(0deg)' },
          '100%': { transform: 'rotate(360deg)' },
        },
      },
      animation: {
        'fade-in-up': 'fadeInUp 0.5s ease forwards',
        'fade-in': 'fadeIn 0.3s ease forwards',
        'scale-in': 'scaleIn 0.3s ease forwards',
        'slide-in-right': 'slideInRight 0.3s ease forwards',
        'slide-in-left': 'slideInLeft 0.3s ease forwards',
        'float': 'float 20s ease-in-out infinite',
        'pulse': 'pulse 2s ease-in-out infinite',
        'spin': 'spin 0.8s linear infinite',
      },

      /* ============================================
         Z-Index 层级扩展
         ============================================ */
      zIndex: {
        'dropdown': '100',
        'sticky': '200',
        'modal-backdrop': '300',
        'modal': '400',
        'tooltip': '500',
        'toast': '600',
      },
    },
  },
  plugins: [
    /* ============================================
       自定义插件 - 玻璃态工具类
       ============================================ */
    function({ addComponents, addUtilities }) {
      // 添加玻璃态组件
      addComponents({
        '.glass': {
          background: 'rgba(255, 255, 255, 0.06)',
          backdropFilter: 'blur(40px)',
          WebkitBackdropFilter: 'blur(40px)',
          border: '1px solid rgba(255, 255, 255, 0.1)',
        },
      });

      // 添加文字工具类
      addUtilities({
        '.text-balance': {
          textWrap: 'balance',
        },
        '.text-shadow': {
          textShadow: '0 1px 2px rgba(0, 0, 0, 0.2)',
        },
      });
    },
  ],
};
