"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import { buttonClass } from "@/components/Button";
import { getToken } from "@/lib/auth/session";

// SCR-002 ログイン。

/**
 * error の値 → 表示する文言（イベント設計 SCR-002）。
 * 文言はこの表から選び、クエリの文字列をそのまま画面に出さない。
 * 任意の文言を表示させられると、偽の案内を出すフィッシングに使われるため。
 */
const ERROR_MESSAGES: Record<string, string> = {
  ACCESS_DENIED: "Google でのアクセスの許可が必要です。もう一度ログインしてください。",
  INVALID_STATE: "ログインをやり直してください。",
  VALIDATION_ERROR: "ログインをやり直してください。",
  OAUTH_EXCHANGE_FAILED: "Google との通信に失敗しました。時間をおいてもう一度お試しください。",
  TOKEN_EXPIRED: "ログインの有効期限が切れました。もう一度ログインしてください。",
  INVALID_TOKEN: "もう一度ログインしてください。",
};
const FALLBACK_MESSAGE = "ログインに失敗しました。もう一度お試しください。";

export function LoginView() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const errorCode = searchParams.get("error");
  const errorMessage = errorCode ? (ERROR_MESSAGES[errorCode] ?? FALLBACK_MESSAGE) : null;
  const [redirecting, setRedirecting] = useState(false);

  // ログイン済みなら画面を出さず取引一覧へ（#1）。期限切れのトークンでも、
  // 共通レイアウトの 401 で破棄されてここへ戻るため、行き来は繰り返さない
  useEffect(() => {
    if (getToken()) router.replace("/transactions");
  }, [router]);

  // Google の画面から「戻る」で戻ったとき、ページが bfcache から復元されると
  // ボタンが押せないままになるため、押せる状態に戻す
  useEffect(() => {
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted) setRedirecting(false);
    };
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, []);

  // ログイン開始（#2）。fetch ではなくブラウザごと API-002 へ移る（Next.js の画面ではなく、
  // プロキシ経由で API サーバに届き、Google の同意画面へ 302 で送られる）。
  // 押した直後にボタンを無効にし、二重に押して state が 2 つ発行されるのを防ぐ
  const onLogin = () => {
    setRedirecting(true);
    const url = new URL("/api/v1/auth/google", window.location.origin);
    url.searchParams.set("redirect_uri", `${window.location.origin}/auth/callback`);
    window.location.assign(url.toString());
  };

  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-10">
      <div className="flex w-full max-w-[380px] flex-col gap-5 rounded-lg border border-line bg-surface p-7">
        <h1 className="text-2xl font-bold">自動家計簿</h1>
        <p className="text-sm leading-relaxed">カードや QR コード決済の通知メールから、毎日の支出を自動で記録します。</p>

        {/* エラーが無いときは、空白も取らない（SCR-002 の状態「通常」） */}
        {errorMessage && (
          <p role="alert" className="rounded-md border border-danger bg-danger-soft px-3 py-2 text-sm text-danger">
            {errorMessage}
          </p>
        )}

        <button type="button" onClick={onLogin} disabled={redirecting} className={buttonClass("primary", "w-full")}>
          {redirecting ? "Google に移動しています…" : "Google でログイン"}
        </button>

        <div className="flex flex-col gap-2 text-xs leading-relaxed text-muted">
          {/* ログインではメールの読み取りを求めない設計（Step 4）を、画面でも伝える */}
          <p>
            ログインで受け取るのは、Google アカウントのメールアドレスと名前だけです。
            メールの読み取りは、あとでメール連携を設定するときに改めて確認します。
          </p>
          {/* トークンを sessionStorage に置くため、新しいタブではログインし直しになる（Step 4 のフィードバック） */}
          <p>ログインは、このタブの中だけで有効です。新しいタブで開いたときは、もう一度ログインしてください。</p>
        </div>
      </div>
    </main>
  );
}
