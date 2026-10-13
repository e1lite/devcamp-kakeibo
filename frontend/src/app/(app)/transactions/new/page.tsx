import type { Metadata } from "next";

import { NewTransactionView } from "@/app/(app)/transactions/new/NewTransactionView";

export const metadata: Metadata = { title: "取引の登録" };

export default function NewTransactionPage() {
  return <NewTransactionView />;
}
