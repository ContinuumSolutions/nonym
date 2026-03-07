/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'primary': '#635bff',
        'primary-light': '#7c73ff',
        'success': '#00d924',
        'warning': '#ff5722',
        'danger': '#dc2626',
        'neutral': {
          50: '#fafbfc',
          100: '#f6f9fc',
          200: '#e3e8ee',
          600: '#556987',
          700: '#3c4257',
          800: '#1a1f2e',
          900: '#0a0e1a',
        }
      },
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
