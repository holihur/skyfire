/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        page: 'rgb(var(--c-page) / <alpha-value>)',
        header: 'rgb(var(--c-header) / <alpha-value>)',
        panel: 'rgb(var(--c-panel) / <alpha-value>)',
        panel2: 'rgb(var(--c-panel2) / <alpha-value>)',
        inset: 'rgb(var(--c-inset) / <alpha-value>)',
        edge: 'rgb(var(--c-edge) / <alpha-value>)',
        edge2: 'rgb(var(--c-edge2) / <alpha-value>)',
        edge3: 'rgb(var(--c-edge3) / <alpha-value>)',
        hover: 'rgb(var(--c-hover) / <alpha-value>)',
        fg: 'rgb(var(--c-fg) / <alpha-value>)',
        fg2: 'rgb(var(--c-fg2) / <alpha-value>)',
        muted: 'rgb(var(--c-muted) / <alpha-value>)',
        faint: 'rgb(var(--c-faint) / <alpha-value>)',
        shade: 'rgb(var(--c-shade) / <alpha-value>)',
        ok: 'rgb(var(--c-ok) / <alpha-value>)',
        warn: 'rgb(var(--c-warn) / <alpha-value>)',
        err: 'rgb(var(--c-err) / <alpha-value>)',
        info: 'rgb(var(--c-info) / <alpha-value>)',
        brand: 'rgb(var(--c-brand) / <alpha-value>)',
        surface: {
          DEFAULT: '#0b1220',
          soft: '#0f172a',
          card: '#111c33',
          line: '#1e2a45',
        },
        accent: {
          DEFAULT: '#38bdf8',
          soft: '#0ea5e9',
        },
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
    },
  },
  plugins: [],
}