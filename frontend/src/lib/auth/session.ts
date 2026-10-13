// ログインのトークンと、ログイン後の戻り先を sessionStorage に置く。
//
// sessionStorage はタブごとのため、新しいタブではログインし直しになる（Step 4 判断 9。
// ログイン画面で先に案内している）。localStorage と違い、タブを閉じれば消える。

const TOKEN_KEY = "kakeibo.access_token";
const RETURN_TO_KEY = "kakeibo.return_to";

// プライベートブラウズなどで sessionStorage が使えない場合も、画面を落とさない
function storage(): Storage | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function getToken(): string | null {
  return storage()?.getItem(TOKEN_KEY) ?? null;
}

export function setToken(token: string): void {
  storage()?.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  storage()?.removeItem(TOKEN_KEY);
}

/**
 * ログイン後の戻り先として使ってよいパスかを返す。
 *
 * `/` で始まり `//` で始まらないパスだけを使う（Step 4 判断 7）。
 * `//evil.example` や `https://...` を受け付けると、ログイン後に外部サイトへ
 * 送られるオープンリダイレクトになるため。
 */
export function isSafeReturnPath(path: string): boolean {
  return path.startsWith("/") && !path.startsWith("//") && !path.startsWith("/\\");
}

// ログイン画面・ログイン処理中の画面へ戻っても意味が無いため、戻り先にしない
const NOT_RETURNABLE = ["/login", "/auth/callback"];

export function saveReturnTo(path: string): void {
  if (!isSafeReturnPath(path)) return;
  if (NOT_RETURNABLE.some((p) => path === p || path.startsWith(`${p}?`))) return;
  storage()?.setItem(RETURN_TO_KEY, path);
}

/** 戻り先を取り出して消す。使ってよいパスでなければ null。 */
export function consumeReturnTo(): string | null {
  const s = storage();
  const path = s?.getItem(RETURN_TO_KEY) ?? null;
  s?.removeItem(RETURN_TO_KEY);
  return path !== null && isSafeReturnPath(path) ? path : null;
}
