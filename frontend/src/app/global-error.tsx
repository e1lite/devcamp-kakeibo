"use client";

import { ErrorScreen, UNEXPECTED_ERROR } from "@/components/ErrorScreen";

import "./globals.css";

/** ルートのレイアウトで例外が起きたとき。ルートのレイアウトを置き換えるため、html と body を持つ。 */
export default function GlobalError() {
  return (
    <html lang="ja">
      <body>
        <title>問題が発生しました | 自動家計簿</title>
        <ErrorScreen {...UNEXPECTED_ERROR} />
      </body>
    </html>
  );
}
