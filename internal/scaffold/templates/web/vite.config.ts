import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
    origin: "http://127.0.0.1:4173",
  },
  build: {
    manifest: true,
  },
});
