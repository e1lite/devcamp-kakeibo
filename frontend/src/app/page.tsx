"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { getToken } from "@/lib/auth/session";

/**
 * SCR-001 ルート。表示は持たず、振り分けだけを行う（イベント設計 SCR-001 #1）。
 *
 * トークンがあれば取引一覧、無ければログインへ（履歴を置き換える）。
 * トークンが有効かは、遷移先の共通レイアウトで API-004 を呼んで確かめる（0 章 #1）。
 */
export default function RootPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace(getToken() ? "/transactions" : "/login");
  }, [router]);

  return null;
}
