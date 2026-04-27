import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: ["./src/**/*.{js,ts,jsx,tsx,mdx}"],
  theme: {
    extend: {
      colors: {
        base: {
          50: "#f8fbfa",
          100: "#eef6f3",
          900: "#0f1916"
        },
        accent: {
          500: "#118a7e",
          600: "#0d6f66"
        },
        warm: {
          500: "#f2a65a"
        }
      },
      boxShadow: {
        soft: "0 12px 40px -24px rgba(0,0,0,0.45)"
      }
    }
  },
  plugins: []
};

export default config;
