// 未サインインで招待リンクを開いたときの招待（要件3-4）。
// サインイン → 表示名の設定（初回のみ）のあとに、同じ参加の確認（INV-12）へ進めるために覚えておく。
// アプリを終了したら忘れてよい（もう一度リンクを開けばよい）ので、端末には保存しない
let pendingToken: string | null = null;

export function rememberPendingInvite(tokenOrCode: string): void {
  pendingToken = tokenOrCode;
}

export function clearPendingInvite(): void {
  pendingToken = null;
}

/** 開いている招待があるか（サインインの画面に、招待の説明を添えるため） */
export function hasPendingInvite(): boolean {
  return pendingToken !== null;
}

/** サインイン・表示名の設定のあとに開く画面。開いた招待があれば、その参加の確認 */
export function homeHref(): '/today' | `/invite/${string}` {
  return pendingToken === null ? '/today' : `/invite/${pendingToken}`;
}
