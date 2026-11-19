// 招待リンクは https://<招待用ドメイン>/i/<トークン>（api/openapi.yaml の CreatedInvitation.url）。
// トークンは URL にそのまま入れられる文字（base64url）だけでできている
const invitePathPattern = /^\/i\/([A-Za-z0-9_-]+)$/;

function pathOf(pathOrUrl: string): string {
  try {
    return new URL(pathOrUrl, 'https://placeholder.invalid').pathname;
  } catch {
    return pathOrUrl;
  }
}

/**
 * 招待リンクで開かれたとき、アプリの参加の確認（/invite/<トークン>）へ読み替える。
 * Expo Router の +native-intent から呼ぶ。ほかのパスはそのまま返す
 */
export function rewriteInvitePath(path: string): string {
  const match = invitePathPattern.exec(pathOf(path));
  return match ? `/invite/${match[1]}` : path;
}

/**
 * QR コードの中身（INV-11）から、招待のトークンを取り出す。招待のリンクでなければ null。
 * 招待用ドメインはまだ決まっていない（タスク 15.3）ので、ドメインは問わず、パスの形で判定する。
 * 別のドメインの /i/... を読んでも、API が「無効な招待」を返すだけで害はない
 */
export function inviteTokenFromScan(data: string): string | null {
  let url: URL;
  try {
    url = new URL(data.trim());
  } catch {
    return null;
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') return null;
  return invitePathPattern.exec(url.pathname)?.[1] ?? null;
}
