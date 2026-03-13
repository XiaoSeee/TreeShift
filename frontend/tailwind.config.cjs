/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ember: {
          50: "#f9f4ee",
          100: "#f1e7d9",
          200: "#e8d3b5",
          300: "#dcb686",
          400: "#ca8f59",
          500: "#ba7240",
          600: "#a05935",
          700: "#7f442c",
          800: "#673928",
          900: "#553023"
        },
        moss: {
          500: "#3d5a41",
          700: "#2e4532"
        },
        ink: {
          900: "#1f1a16"
        }
      },
      boxShadow: {
        float: "0 24px 60px rgba(58, 35, 17, 0.14)"
      },
      fontFamily: {
        display: ["'Space Grotesk'", "'Bahnschrift'", "'Segoe UI'", "sans-serif"],
        body: ["'IBM Plex Sans'", "'Segoe UI'", "sans-serif"]
      }
    }
  },
  plugins: []
};
