import type { Metadata } from "next";

import { TransactionDetailView } from "@/app/(app)/transactions/[id]/TransactionDetailView";

export const metadata: Metadata = { title: "取引の詳細" };

export default async function TransactionDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <TransactionDetailView id={id} />;
}
