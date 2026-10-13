import type { Metadata } from "next";

import { CategoriesView } from "@/app/(app)/settings/categories/CategoriesView";

export const metadata: Metadata = { title: "カテゴリ" };

export default function CategoriesPage() {
  return <CategoriesView />;
}
