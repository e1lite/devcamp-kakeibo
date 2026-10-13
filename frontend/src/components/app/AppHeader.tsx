"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { logout } from "@/lib/api/endpoints";
import type { Me } from "@/lib/api/types";
import { clearToken } from "@/lib/auth/session";

// 共通レイアウトの A ヘッダー（画面上部に固定）。
// PC は 1 行に サービス名・ナビ・表示名・ログアウト。スマホは表示名を省き、
// ログアウトを設定メニューの末尾へ移す（Step 5 の画面レイアウト「レスポンシブ」）。

function displayName(user: Me): string {
  return user.display_name ?? user.email;
}

const navLinkClass =
  "inline-flex min-h-11 items-center px-3 text-sm aria-[current=page]:font-bold " +
  "aria-[current=page]:underline aria-[current=page]:underline-offset-8 aria-[current=page]:decoration-2";

export function AppHeader({ user }: { user: Me | null }) {
  const pathname = usePathname();
  const router = useRouter();
  const [loggingOut, setLoggingOut] = useState(false);

  // ログアウト（共通 #4）。結果にかかわらずトークンを破棄してログイン画面へ。戻り先は保存しない
  const onLogout = async () => {
    setLoggingOut(true);
    try {
      await logout();
    } catch {
      // サーバ側に失効リストは無く、ログアウトの実体はトークンの破棄（API-005）。失敗しても進める
    }
    clearToken();
    router.replace("/login");
  };

  return (
    <header className="sticky top-0 z-20 border-b border-line bg-surface">
      <div className="mx-auto flex h-14 max-w-[960px] items-center gap-2 px-4">
        {/* どの画面からでも取引一覧（ホーム）へ戻れるようにする（共通 #2） */}
        <Link href="/transactions" className="mr-auto inline-flex min-h-11 items-center font-bold md:mr-4">
          自動家計簿
        </Link>

        {user && (
          <>
            <nav aria-label="メイン" className="flex items-center">
              <Link
                href="/transactions"
                className={navLinkClass}
                aria-current={pathname.startsWith("/transactions") ? "page" : undefined}
              >
                取引
              </Link>
              <SettingsMenu
                pathname={pathname}
                userName={displayName(user)}
                onLogout={onLogout}
                loggingOut={loggingOut}
              />
            </nav>

            <div className="ml-auto hidden items-center gap-3 md:flex">
              <span className="text-sm text-muted">{displayName(user)}</span>
              <button
                type="button"
                onClick={onLogout}
                disabled={loggingOut}
                className="inline-flex min-h-11 items-center text-sm underline underline-offset-4 disabled:opacity-50"
              >
                ログアウト
              </button>
            </div>
          </>
        )}
      </div>
    </header>
  );
}

type SettingsMenuProps = {
  pathname: string;
  userName: string;
  onLogout: () => void;
  loggingOut: boolean;
};

/**
 * 設定メニュー（共通 #3）。コアでは「カテゴリ」のみだが、拡張で設定項目が増えるため
 * 最初からメニューにまとめておく。外側の押下・Esc・画面の移動で閉じる。
 */
function SettingsMenu({ pathname, userName, onLogout, loggingOut }: SettingsMenuProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  // 画面を移ったら閉じる
  const [lastPathname, setLastPathname] = useState(pathname);
  if (pathname !== lastPathname) {
    setLastPathname(pathname);
    setOpen(false);
  }

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        buttonRef.current?.focus();
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={buttonRef}
        type="button"
        aria-expanded={open}
        aria-controls="settings-menu"
        onClick={() => setOpen((v) => !v)}
        className={navLinkClass}
        aria-current={pathname.startsWith("/settings") ? "page" : undefined}
      >
        設定 ▾
      </button>
      {open && (
        <div
          id="settings-menu"
          className="absolute right-0 z-30 mt-1 flex w-56 flex-col rounded-md border border-line bg-surface py-1 shadow-lg md:left-0 md:right-auto"
        >
          <Link
            href="/settings/categories"
            className="flex min-h-11 items-center px-4 text-sm hover:bg-bg aria-[current=page]:font-bold"
            aria-current={pathname === "/settings/categories" ? "page" : undefined}
          >
            カテゴリ
          </Link>
          <Link
            href="/settings/mail-accounts"
            className="flex min-h-11 items-center px-4 text-sm hover:bg-bg aria-[current=page]:font-bold"
            aria-current={pathname === "/settings/mail-accounts" ? "page" : undefined}
          >
            メール連携
          </Link>
          {/* スマホではヘッダーに表示名とログアウトを出さないため、メニューの末尾に置く */}
          <div className="mt-1 border-t border-line pt-1 md:hidden">
            <p className="px-4 py-2 text-xs text-muted">{userName} でログイン中</p>
            <button
              type="button"
              onClick={onLogout}
              disabled={loggingOut}
              className="flex min-h-11 w-full items-center px-4 text-left text-sm underline underline-offset-4 disabled:opacity-50"
            >
              ログアウト
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
