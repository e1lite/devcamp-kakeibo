"use client";

import { useCallback, useEffect, useState } from "react";

// 画面を開いたときの取得（カテゴリ・決済手段・取引 1 件など）の状態。
//
// 取引・カテゴリなどのデータは画面ごとに取得し、操作のあとは取り直す。
// 状態管理のライブラリやキャッシュは使わない（Step 5 の状態管理の想定）。

export type Resource<T> =
  | { status: "loading" }
  | { status: "success"; data: T }
  | { status: "error"; error: unknown };

export type ResourceHandle<T> = {
  state: Resource<T>;
  /** 読み込み中の表示に戻して取り直す（「再試行」）。 */
  retry: () => void;
  /** 今の表示を残したまま取り直す（カテゴリ管理の、作成・更新・削除のあと）。 */
  refresh: () => void;
};

/**
 * load を呼んで結果を持つ。load が変わる（依存する値が変わる）と取り直す。
 * load は useCallback で包んで渡すこと。
 */
export function useResource<T>(load: () => Promise<T>): ResourceHandle<T> {
  const [state, setState] = useState<Resource<T>>({ status: "loading" });
  const [version, setVersion] = useState(0);

  // load が変わったら、描画中に読み込み中へ戻す（effect の中で state を同期的に書き換えない）
  const [lastLoad, setLastLoad] = useState(() => load);
  if (load !== lastLoad) {
    setLastLoad(() => load);
    setState({ status: "loading" });
  }

  useEffect(() => {
    // 取得の途中で条件が変わったら、前の取得の結果は捨てる
    let active = true;
    load().then(
      (data) => {
        if (active) setState({ status: "success", data });
      },
      (error: unknown) => {
        if (active) setState({ status: "error", error });
      },
    );
    return () => {
      active = false;
    };
  }, [load, version]);

  const retry = useCallback(() => {
    setState({ status: "loading" });
    setVersion((v) => v + 1);
  }, []);

  const refresh = useCallback(() => setVersion((v) => v + 1), []);

  return { state, retry, refresh };
}
