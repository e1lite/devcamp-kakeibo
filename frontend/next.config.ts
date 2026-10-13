import type { NextConfig } from "next";

// API サーバの起点。/api/* をここへ転送する（API 仕様書 2 章）。
// ブラウザからは同一オリジン（localhost:5173）の /api/* として呼ぶため、
// API サーバは CORS ヘッダを返さなくてよい。
const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  // cacheComponents（Partial Prerendering と Activity による画面の保持）は使わない。
  // 全画面が sessionStorage のトークンで API を呼ぶ Client Component のため、
  // サーバ側の事前描画・キャッシュの利点が無い。また Activity は離れた画面の
  // 入力途中の内容を残すため、「開くたびに取り直す」「登録後は空のフォーム」という
  // イベント設計（Step 5）と食い違う。partialPrefetching は cacheComponents が前提のため外す。
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiOrigin}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
