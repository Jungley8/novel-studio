/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        atelier: {
          950: '#0b0d11',
          900: '#11141a',
          850: '#161a22',
          800: '#1d222d',
          750: '#252b38',
          700: '#303848',
          600: '#424b5d',
        },
        ink: {
          50: '#f8f9fa',
          100: '#e5e7eb',
          200: '#d1d5db',
          300: '#9ca3af',
          400: '#6b7280',
          500: '#4b5563',
          600: '#374151',
        },
        parchment: {
          50: '#fdfbf7',
          100: '#f8f4eb',
          200: '#efe6d3',
          800: '#3e3423',
          900: '#272014',
        },
        brand: {
          amber: '#e5a93c',
          'amber-hover': '#f59e0b',
          'amber-light': '#fef3c7',
          emerald: '#10b981',
          rose: '#e11d48',
          cyan: '#06b6d4',
          indigo: '#6366f1',
        }
      },
      fontFamily: {
        serif: ['"Noto Serif SC"', '"Source Han Serif SC"', '"Songti SC"', 'Georgia', 'serif'],
        sans: ['"Inter"', '-apple-system', 'BlinkMacSystemFont', '"PingFang SC"', '"Hiragino Sans GB"', 'sans-serif'],
        mono: ['"JetBrains Mono"', '"SF Mono"', 'Menlo', 'Monaco', 'monospace'],
      },
      boxShadow: {
        'atelier-sm': '0 1px 3px rgba(0, 0, 0, 0.4), 0 1px 2px rgba(0, 0, 0, 0.3)',
        'atelier-md': '0 4px 12px rgba(0, 0, 0, 0.5), 0 2px 4px rgba(0, 0, 0, 0.4)',
        'atelier-lg': '0 12px 32px rgba(0, 0, 0, 0.7), 0 4px 8px rgba(0, 0, 0, 0.5)',
        'amber-glow': '0 0 20px rgba(229, 169, 60, 0.15)',
      }
    },
  },
  plugins: [],
};
