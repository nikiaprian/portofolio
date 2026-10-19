/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./public/templates/**/*.html",
    "./db/seeds/layouts/**/*.html",
    "./assets/js/**/*.js"
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        accent: "var(--accent-color, #08285a)",
      },
      fontFamily: {
        serif: ["Georgia", "serif"],
        sans: ["Inter", "Arial", "sans-serif"],
      },
      animation: {
        fadeUp: "fadeUp .7s ease forwards",
        slideDown: "slideDown .35s ease forwards",
      },
      keyframes: {
        fadeUp: {
          "0%": { opacity: "0", transform: "translateY(24px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        slideDown: {
          "0%": { opacity: "0", transform: "translateY(-8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
    },
  },
  plugins: [
    function ({ addUtilities }) {
      addUtilities({
        ".scrollbar-none": {
          "scrollbar-width": "none",
          "-ms-overflow-style": "none",
        },
        ".scrollbar-none::-webkit-scrollbar": {
          display: "none",
        },
      });
    },
  ],
}
