import type { Metadata } from "next";

import { ErrorScreen, NOT_FOUND_PAGE } from "@/components/ErrorScreen";

export const metadata: Metadata = { title: "ページが見つかりません" };

/** SCR-099 エラー（存在しない URL）。 */
export default function NotFound() {
  return <ErrorScreen {...NOT_FOUND_PAGE} />;
}
