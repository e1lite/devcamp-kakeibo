import type { Metadata } from "next";
import { Suspense } from "react";

import { LoginView } from "@/app/login/LoginView";

export const metadata: Metadata = { title: "ログイン" };

// useSearchParams を使う Client Component は Suspense で囲む（囲まないとビルドで失敗する）
export default function LoginPage() {
  return (
    <Suspense>
      <LoginView />
    </Suspense>
  );
}
