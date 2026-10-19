/** @type {import('tailwindcss').Config} */
import PrimeUI from 'tailwindcss-primeui';

export default {
  darkMode: ['selector', '[class*="app-dark"]'],
  content: [
    './index.html',
    './public/**/*.json',
    './src/**/*.{html,ts,scss,js}',
    './node_modules/photoswipe/**/*.js',
  ],
  plugins: [PrimeUI],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
      },
      colors: {
        // Cluvix Brand Colors
        // Primary: purple — màu chủ đạo duy nhất
        // Accent: orange — badge/highlight, không dùng cho button/nav
        // Danger: red — error/destructive only
        // Burgundy: gradient partner với purple, không dùng độc lập trong UI
        cluvix: {
          primary: {
            DEFAULT: '#7B3F9E',
            light: '#9B5FBE',
            dark: '#6A3589',
          },
          accent: {
            DEFAULT: '#FF5722',
            light: '#FF7043',
            dark: '#F4511E',
          },
          danger: {
            DEFAULT: '#E53935',
            light: '#EF5350',
            dark: '#D32F2F',
          },
          gradient: '#8B1538', // chỉ dùng trong gradient brand, không dùng độc lập
        },
      },
    },
    screens: {
      sm: '576px',
      md: '768px',
      lg: '992px',
      xl: '1200px',
      '2xl': '1920px',
    },
  },
};
