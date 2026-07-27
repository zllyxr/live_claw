import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import uniPlugin from "@dcloudio/vite-plugin-uni";

const uni = typeof uniPlugin === "function" ? uniPlugin : (uniPlugin as any).default;
const H5_BASE = "/h5/";
const LOCAL_CORE_PROXY = process.env.VITE_CORE_PROXY_TARGET;
const CORE_PROXY_TARGET = LOCAL_CORE_PROXY || "https://live.travelig.vip";

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
        target: "https://live.travelig.vip",
        changeOrigin: true,
        secure: false
      },
      "/core-api": {
        target: CORE_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        rewrite: LOCAL_CORE_PROXY ? (path) => path.replace(/^\/core-api/, "") : undefined
      }
    }
  },
  resolve: {
    alias: {
      "@openim/protocol/lib/pb/sdkws/sdkws": fileURLToPath(new URL("./src/vendor/openim-sdkws.ts", import.meta.url)),
      "@": fileURLToPath(new URL("./src", import.meta.url))
    }
  }
}));
