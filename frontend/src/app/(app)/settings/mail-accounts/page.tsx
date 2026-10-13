import type { Metadata } from "next";

import { MailAccountsView } from "@/app/(app)/settings/mail-accounts/MailAccountsView";

export const metadata: Metadata = { title: "メール連携" };

export default function MailAccountsPage() {
  return <MailAccountsView />;
}
