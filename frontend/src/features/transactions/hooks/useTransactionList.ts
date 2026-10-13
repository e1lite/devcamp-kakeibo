"use client";

import { useEffect, useState } from "react";

import { toApiParams, type ListQuery } from "@/features/transactions/listQuery";
import { ApiError, isAbort } from "@/lib/api/client";
import { listTransactions } from "@/lib/api/endpoints";
import type { TransactionListMeta, TransactionSummary } from "@/lib/api/types";

// 取引一覧（SCR-010）の取得。API-010 を呼ぶのはこの hook だけ（イベント設計 SCR-010 #1）。
//
// 絞り込みの操作（期間・カテゴリ・キーワード・ページ送り）は URL を書き換えるだけにし、
// 「URL が変わったら取得する」をここに 1 つだけ置く（Step 5 のイベント設計の方針 #1）。
// 画面の側には fetch を書かない。理由は ../README.md を参照。

export type TransactionListState =
  | { status: "loading" }
  | { status: "success"; rows: TransactionSummary[]; meta: TransactionListMeta }
  // 400 VALIDATION_ERROR / INVALID_DATE_RANGE（URL を手で書き換えた場合など）
  | { status: "invalid" }
  // 5xx・通信エラー
  | { status: "error" };

export function useTransactionList(query: ListQuery): { state: TransactionListState; retry: () => void } {
  // API に渡す値を文字列にして、変わったときだけ取り直す
  const apiQuery = toApiParams(query).toString();
  const [version, setVersion] = useState(0);
  const [state, setState] = useState<TransactionListState>({ status: "loading" });

  // 条件が変わったら、描画中に読み込み中へ戻す（集計と一覧の位置にスケルトンを出す）
  const requestKey = `${apiQuery}#${version}`;
  const [lastRequestKey, setLastRequestKey] = useState(requestKey);
  if (requestKey !== lastRequestKey) {
    setLastRequestKey(requestKey);
    setState({ status: "loading" });
  }

  useEffect(() => {
    // 取得の途中で条件が変わったら、前の取得は中断して結果を捨てる。
    // 素早く条件を変えたとき、遅れて返った古い結果で一覧が上書きされないように（イベント設計 SCR-010）
    const controller = new AbortController();

    listTransactions(new URLSearchParams(apiQuery), controller.signal).then(
      ({ data, meta }) => setState({ status: "success", rows: data, meta }),
      (error: unknown) => {
        if (isAbort(error)) return;
        if (error instanceof ApiError && error.status === 400) setState({ status: "invalid" });
        else setState({ status: "error" });
      },
    );

    return () => controller.abort();
  }, [apiQuery, version]);

  return { state, retry: () => setVersion((v) => v + 1) };
}
