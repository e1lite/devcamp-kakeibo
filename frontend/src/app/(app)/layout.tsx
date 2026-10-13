import { AppShell } from "@/components/app/AppShell";

/**
 * 認証が必要な画面の共通レイアウト（SCR-001 / 002 / 003 / 099 以外のすべて）。
 * (app) はルートのグループで、URL には現れない。
 */
export default function AppLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <AppShell>{children}</AppShell>;
}
