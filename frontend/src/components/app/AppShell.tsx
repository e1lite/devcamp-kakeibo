"use client";

import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { AppHeader } from "@/components/app/AppHeader";
import { LoadError } from "@/components/Feedback";
import { getMe } from "@/lib/api/endpoints";
import type { Me } from "@/lib/api/types";
import { getToken, saveReturnTo } from "@/lib/auth/session";

// 共通レイアウト（認証が必要な画面すべて）。イベント設計 1.1。
//
// Next.js の layout は画面を移っても作り直されないため、ログインの確認（API-004）は
// ページを読み込んだときに 1 回だけ行われる（イベント設計 0 章 #1）。
// 画面をまたいで共有する状態は、ログイン中のユーザと完了メッセージだけで、ここに置く
// （Step 5 の状態管理の想定）。

type FlashTarget = "next" | "here";

type AppContextValue = {
  user: Me;
  /**
   * 完了メッセージを出す。"next" は遷移先の画面で、"here" は今の画面で出す。
   * 遷移先で 1 回だけ出し、リロードでは再び出さない（イベント設計 1.2）。
   */
  flash: (message: string, target?: FlashTarget) => void;
  /** 履歴を戻る。アプリの中で戻れる履歴が無ければ fallback へ（イベント設計 SCR-011 #4 など）。 */
  back: (fallback: string) => void;
  /** 履歴を積まずに移る。戻るの判断に使う履歴の深さを変えない。 */
  replace: (href: string) => void;
};

const AppContext = createContext<AppContextValue | null>(null);

export function useApp(): AppContextValue {
  const value = useContext(AppContext);
  if (!value) throw new Error("useApp は AppShell の中で使ってください");
  return value;
}

type FlashState = { message: string; from: string; shownOn: string | null };

const FLASH_MS = 4000;

type AuthState = { status: "checking" } | { status: "ready"; user: Me } | { status: "error" };

// --- 戻れる履歴の深さ ----------------------------------------------------------
//
// App Router は履歴の位置を教えてくれないため、アプリの中での画面の移動を数えておく。
// ブラウザの「戻る」（popstate）なら 1 つ減らし、それ以外の移動なら 1 つ増やす。
// replace で移ったときは履歴が増えないため数えない。
// 履歴はウィンドウに 1 つだけのため、コンポーネントの state ではなくここに持つ。
const history = { depth: 0, popped: false, replacing: false, previousPath: null as string | null };

export function AppShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [auth, setAuth] = useState<AuthState>({ status: "checking" });
  const [authAttempt, setAuthAttempt] = useState(0);

  // --- 認証の確認（共通 #1） ----------------------------------------------

  useEffect(() => {
    if (!getToken()) {
      // トークンが無ければ API を呼ばず、開こうとしていた URL を戻り先にしてログインへ
      saveReturnTo(window.location.pathname + window.location.search);
      router.replace("/login");
      return;
    }
    let active = true;
    getMe().then(
      (user) => {
        if (active) setAuth({ status: "ready", user });
      },
      // 401 は API クライアントがログイン画面へ送る。ここに来るのは 5xx・通信エラー
      () => {
        if (active) setAuth({ status: "error" });
      },
    );
    return () => {
      active = false;
    };
  }, [router, authAttempt]);

  const retryAuth = () => {
    setAuth({ status: "checking" });
    setAuthAttempt((n) => n + 1);
  };

  // --- 戻れる履歴の深さ ------------------------------------------------------

  useEffect(() => {
    // ログアウトしてログインし直したときなど、共通レイアウトを開き直したら数え直す
    Object.assign(history, { depth: 0, popped: false, replacing: false, previousPath: null });
    const onPop = () => {
      history.popped = true;
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  useEffect(() => {
    if (history.previousPath !== null && history.previousPath !== pathname) {
      if (history.popped) history.depth = Math.max(0, history.depth - 1);
      else if (!history.replacing) history.depth += 1;
    }
    history.popped = false;
    history.replacing = false;
    history.previousPath = pathname;
  }, [pathname]);

  const back = useCallback(
    (fallback: string) => {
      if (history.depth > 0) router.back();
      else {
        history.replacing = true;
        router.replace(fallback);
      }
    },
    [router],
  );

  const replace = useCallback(
    (href: string) => {
      history.replacing = true;
      router.replace(href);
    },
    [router],
  );

  // --- 完了メッセージ --------------------------------------------------------

  const [flashState, setFlashState] = useState<FlashState | null>(null);

  const flash = useCallback((message: string, target: FlashTarget = "next") => {
    const here = window.location.pathname;
    setFlashState({ message, from: here, shownOn: target === "here" ? here : null });
  }, []);

  // 遷移先に着いたら、その画面で表示を始める。さらに別の画面へ移ったら消す。
  // 画面が移ったことは描画中に前回の pathname と比べて知る（effect で state を書き換えない）
  const [lastPathname, setLastPathname] = useState(pathname);
  if (pathname !== lastPathname) {
    setLastPathname(pathname);
    if (flashState?.shownOn === null && pathname !== flashState.from) {
      setFlashState({ ...flashState, shownOn: pathname });
    } else if (flashState?.shownOn != null && pathname !== flashState.shownOn) {
      setFlashState(null);
    }
  }

  const shownMessage = flashState?.shownOn === pathname ? flashState.message : null;

  useEffect(() => {
    if (!shownMessage) return;
    const timer = window.setTimeout(() => setFlashState(null), FLASH_MS);
    return () => window.clearTimeout(timer);
  }, [shownMessage]);

  const value = useMemo<AppContextValue | null>(
    () => (auth.status === "ready" ? { user: auth.user, flash, back, replace } : null),
    [auth, flash, back, replace],
  );

  return (
    <div className="flex min-h-dvh flex-col">
      {/* 確認が済む前は、ヘッダーもサービス名だけにする（共通レイアウトの状態「ログインの確認中」） */}
      <AppHeader user={auth.status === "ready" ? auth.user : null} />
      <main className="w-full flex-1 px-4 pt-6 pb-28 md:pb-10">
        {auth.status === "checking" && <p className="mx-auto max-w-[960px] text-sm text-muted">読み込み中…</p>}
        {auth.status === "error" && (
          <div className="mx-auto max-w-[960px]">
            <LoadError message="ログイン情報を読み込めませんでした。" onRetry={retryAuth} />
          </div>
        )}
        {value && (
          <AppContext.Provider value={value}>
            <FlashContext.Provider value={shownMessage}>{children}</FlashContext.Provider>
          </AppContext.Provider>
        )}
      </main>
    </div>
  );
}

const FlashContext = createContext<string | null>(null);

/** 完了メッセージ（共通レイアウトの B メッセージ）。各画面の内容の先頭に置く。 */
export function FlashMessage() {
  const message = useContext(FlashContext);
  if (!message) return null;
  return (
    <div role="status" className="rounded-md border border-ok bg-ok-soft px-4 py-3 text-sm font-bold text-ok">
      ✓ {message}
    </div>
  );
}

const WIDTH = {
  // 一覧系 960px・詳細 680px・フォーム 520px（共通レイアウトの C 画面の内容）
  list: "max-w-[960px]",
  detail: "max-w-[680px]",
  form: "max-w-[520px]",
} as const;

/** 画面の内容の枠。先頭に完了メッセージの場所を置き、全画面で同じ位置に出す。 */
export function Page({ width, children }: { width: keyof typeof WIDTH; children: ReactNode }) {
  return (
    <div className={`mx-auto flex w-full flex-col gap-5 ${WIDTH[width]}`}>
      <FlashMessage />
      {children}
    </div>
  );
}
