// API の呼び出しを 1 か所にまとめる（Step 5 の講座の振り返り「API の呼び出しを 1 か所にまとめる」）。
//
// - トークンの付与
// - 401 の共通の扱い（イベント設計 1.2）
// - エラーの形の変換（{ error: { code, message } } → ApiError）
// を各画面に書かないようにする。

import { clearToken, getToken, saveReturnTo } from "@/lib/auth/session";

const BASE = "/api/v1";

/** 通信そのものが失敗した（サーバに届かない・応答が壊れている）ことを表すコード。 */
export const NETWORK_ERROR = "NETWORK_ERROR";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * 画面の中で「再試行」させる種類のエラーか（イベント設計 1.2 の 5xx・通信エラー）。
 * 400 / 404 / 409 は画面ごとの表で扱い、こちらには含めない。
 */
export function isRetryable(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 0 || error.status >= 500);
}

export function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  /**
   * 401 でもログイン画面へ送らない。ログアウト（API-005）だけが使う。
   * ログアウトしたいだけの人に「期限切れ」を見せないため（イベント設計 1.2）。
   */
  skipAuthRedirect?: boolean;
};

export type ApiResult<T, M> = { data: T; meta: M };

let redirectingToLogin = false;

/**
 * 401 を受けたら、トークンを破棄し、今の URL を戻り先として保存してログイン画面へ送る。
 *
 * 画面を開いたときに複数の API を並行して呼ぶため、同時に 401 が返りうる。
 * 遷移は 1 回だけにする（イベント設計 1.2）。
 */
function redirectToLogin(code: string): void {
  if (redirectingToLogin) return;
  redirectingToLogin = true;

  clearToken();
  saveReturnTo(window.location.pathname + window.location.search);
  // 期限切れのトークンで作られた画面の状態を引き継がないよう、ページごと移る
  window.location.replace(`/login?error=${encodeURIComponent(code)}`);
}

export async function request<T, M = undefined>(
  path: string,
  { method = "GET", body, signal, skipAuthRedirect = false }: RequestOptions = {},
): Promise<ApiResult<T, M>> {
  const headers: Record<string, string> = { Accept: "application/json" };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";

  let response: Response;
  try {
    response = await fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
      // 登録・削除の直後に古い結果を出さないよう、キャッシュを使わない
      cache: "no-store",
    });
  } catch (error) {
    // 取得の途中で条件が変わって中断した場合は、呼び出し側で捨てる
    if (isAbort(error)) throw error;
    throw new ApiError(0, NETWORK_ERROR, "通信に失敗しました");
  }

  if (response.status === 204) {
    return { data: undefined as T, meta: undefined as M };
  }

  let payload: unknown = null;
  try {
    payload = await response.json();
  } catch {
    // 本文が JSON でない（プロキシのエラー画面など）。下でステータスだけを見る
  }

  if (!response.ok) {
    const error = (payload as { error?: { code?: string; message?: string } } | null)?.error;
    const code = error?.code ?? `HTTP_${response.status}`;
    const message = error?.message ?? `HTTP ${response.status}`;

    if (response.status === 401 && !skipAuthRedirect) {
      redirectToLogin(code);
      // ログイン画面へ移るまで、呼び出し側の画面にエラーを出させない
      return new Promise<never>(() => {});
    }
    throw new ApiError(response.status, code, message);
  }

  if (payload === null || typeof payload !== "object" || !("data" in payload)) {
    throw new ApiError(response.status, NETWORK_ERROR, "応答の形式が正しくありません");
  }
  const { data, meta } = payload as { data: T; meta?: M };
  return { data, meta: meta as M };
}
