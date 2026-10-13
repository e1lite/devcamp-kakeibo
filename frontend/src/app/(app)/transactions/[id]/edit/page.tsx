import type { Metadata } from "next";

import { EditTransactionView } from "@/app/(app)/transactions/[id]/edit/EditTransactionView";

export const metadata: Metadata = { title: "取引の編集" };

export default async function EditTransactionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <EditTransactionView id={id} />;
}
