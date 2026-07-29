import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import uniPlugin from "@dcloudio/vite-plugin-uni";

const uni = typeof uniPlugin === "function" ? uniPlugin : (uniPlugin as any).default;
const H5_BASE = "/h5/";
const V2_PROXY_TARGET =
  process.env.VITE_V2_PROXY_TARGET ||
  process.env.VITE_API_PROXY_TARGET ||
  process.env.VITE_CORE_PROXY_TARGET ||
  "http://127.0.0.1:28080";

export default defineConfig(({ command }) => ({
  base: command === "build" && process.env.UNI_PLATFORM === "h5" ? H5_BASE : "/",
  plugins: [
    uni(),
    {
      name: "xingyu-h5-static-base",
      apply: "build",
      enforce: "post",
      generateBundle(_, bundle) {
        if (process.env.UNI_PLATFORM !== "h5") {
          return;
        }
        const rewrite = (source: string) => source.replace(/(["'`(])\/static\//g, `$1${H5_BASE}static/`);
        Object.values(bundle).forEach((item) => {
          if (item.type === "chunk") {
            item.code = rewrite(item.code);
          } else if (typeof item.source === "string") {
            item.source = rewrite(item.source);
          }
        });
      }
    }
  ],
  server: {
    host: "0.0.0.0",
    proxy: {
      "/appapi": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false
      },
      "/api/v2": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false
      },
      "/claw-public": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false
      },
      "/gameapi": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false
      },
      "/minigame": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false
      },
      "/ws": {
        target: V2_PROXY_TARGET,
        // Keep the browser-facing Host header so the IM server's strict
        // same-origin WebSocket check also succeeds through the H5 dev proxy.
        changeOrigin: false,
        secure: false,
        ws: true
      },
      "/core-api": {
        target: V2_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/core-api/, "")
      }
    }
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url))
    }
  }
}));
