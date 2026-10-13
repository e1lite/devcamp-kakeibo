import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// 画面に依存しない純粋な関数（URL のクエリと API の値の変換、入力の検証、
// 送る値の組み立てなど）を単体テストする。画面の描画はテストの対象にしない。
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    include: ["src/**/*.test.ts"],
    environment: "node",
  },
});
