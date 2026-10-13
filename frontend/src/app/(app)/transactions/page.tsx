import type { Metadata } from "next";
import { Suspense } from "react";

import { TransactionListView } from "@/app/(app)/transactions/TransactionListView";

export const metadata: Metadata = { title: "取引" };

// useSearchParams（絞り込み条件）を使うため Suspense で囲む
export default function TransactionsPage() {
  return (
    <Suspense>
      <TransactionListView />
    </Suspense>
  );
}
