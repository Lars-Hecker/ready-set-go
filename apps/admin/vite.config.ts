import path from "path";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";

const uiPkg = path.resolve(__dirname, "../../package/ui/src");
const rootDir = path.resolve(__dirname, "../..");

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, rootDir, "");
  const apiTarget = env.API_TARGET || "http://localhost:8080";

  return {
    plugins: [
      tanstackRouter({ target: "react", autoCodeSplitting: true }),
      react(),
      tailwindcss(),
    ],
    resolve: {
      alias: [
        { find: "@/components/ui", replacement: path.join(uiPkg, "components") },
        { find: "@/lib/utils", replacement: path.join(uiPkg, "lib/utils") },
        { find: "@/hooks/use-mobile", replacement: path.join(uiPkg, "hooks/use-mobile") },
        { find: "@", replacement: path.resolve(__dirname, "./src") },
      ],
    },
    server: {
      fs: {
        allow: ["../.."],
      },
      proxy: {
        "/api": {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      outDir: "dist",
      emptyOutDir: true,
    },
  };
});
