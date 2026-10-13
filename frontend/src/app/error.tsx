"use client";

import { ErrorScreen, UNEXPECTED_ERROR } from "@/components/ErrorScreen";

/**
 * SCR-099 エラー（画面の処理で想定外の例外が起きたとき）。
 *
 * 共通レイアウトより上に置き、ヘッダーごと差し替える（SCR-099 はヘッダーを出さない）。
 * API の 5xx・通信エラーではここに来ない。その画面の中で「再試行」させる（イベント設計 1.2）。
 */
export default function ErrorPage() {
  return <ErrorScreen {...UNEXPECTED_ERROR} />;
}
