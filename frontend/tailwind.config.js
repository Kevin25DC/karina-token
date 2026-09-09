/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink: {
          950: '#0b0b0f',
          900: '#101018',
          850: '#13131c',
          800: '#171720',
          700: '#1e1e2a',
        },
      },
      fontFamily: {
        sans: [
          'Inter',
          'ui-sans-serif',
          '-apple-system',
          'Segoe UI Variable',
          'Segoe UI',
          'system-ui',
          'sans-serif',
        ],
        mono: [
          'ui-monospace',
          'SFMono-Regular',
          'Cascadia Code',
          'Consolas',
          'monospace',
        ],
      },
      borderRadius: {
        xl2: '1rem',
      },
      boxShadow: {
        card: '0 1px 0 0 rgb(255 255 255 / 0.03) inset, 0 8px 30px rgb(0 0 0 / 0.35)',
        glow: '0 0 0 1px rgb(255 255 255 / 0.06), 0 12px 40px -12px rgb(0 0 0 / 0.6)',
        'glow-sm': '0 0 0 1px rgb(255 255 255 / 0.05), 0 6px 24px -12px rgb(0 0 0 / 0.6)',
      },
      keyframes: {
        'fade-in': {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        'rise-in': {
          from: { opacity: '0', transform: 'translateY(8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'scale-in': {
          from: { opacity: '0', transform: 'scale(0.96)' },
          to: { opacity: '1', transform: 'scale(1)' },
        },
        shimmer: {
          from: { backgroundPosition: '200% 0' },
          to: { backgroundPosition: '-200% 0' },
        },
        'pulse-dot': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.35' },
        },
      },
      animation: {
        'fade-in': 'fade-in 200ms ease-out',
        'rise-in': 'rise-in 260ms cubic-bezier(0.16, 1, 0.3, 1)',
        'scale-in': 'scale-in 180ms cubic-bezier(0.16, 1, 0.3, 1)',
        'pulse-dot': 'pulse-dot 1.6s ease-in-out infinite',
      },
    },
  },
  plugins: [],
};
