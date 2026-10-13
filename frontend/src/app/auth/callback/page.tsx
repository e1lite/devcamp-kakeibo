"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef } from "react";

import { consumeReturnTo, setToken } from "@/lib/auth/session";

/**
 * SCR-003 ログイン処理中。API-003 のリダイレクト先。
 *
 * 表示は「ログインしています…」のみ。いずれも履歴を置き換えて遷移し、
 * ブラウザの「戻る」でこの画面に戻らないようにする（イベント設計 SCR-003）。
 */
export default function AuthCallbackPage() {
  const router = useRouter();
  // 開発時の StrictMode は effect を 2 回実行する。1 回目でフラグメントを消すため、
  // 2 回目は「どちらも無い」と判定してログイン画面へ送ってしまう。処理は 1 回だけにする
  const handled = useRef(false);

  useEffect(() => {
    if (handled.current) return;
    handled.current = true;

    const fragment = new URLSearchParams(window.location.hash.slice(1));
    const token = fragment.get("access_token");
    const error = fragment.get("error");

    // ブラウザの履歴・画面共有にトークンを残さないよう、まずフラグメントを URL から消す
    window.history.replaceState(window.history.state, "", window.location.pathname);

    if (token) {
      // #1 ログイン成功。戻り先があればそこへ（使ったら消す）、無ければ取引一覧へ
      setToken(token);
      router.replace(consumeReturnTo() ?? "/transactions");
    } else if (error && /^[A-Z_]+$/.test(error)) {
      // #2 ログイン失敗。文言はログイン画面で表から選ぶ
      router.replace(`/login?error=${error}`);
    } else {
      // #3 不正な到達（URL を直接開いたなど）
      router.replace("/login");
    }
  }, [router]);

  return (
    <main className="flex min-h-dvh items-center justify-center px-4">
      <div className="w-full max-w-[380px] rounded-lg border border-line bg-surface p-7 text-center">
        <p role="status">ログインしています…</p>
      </div>
    </main>
  );
}
