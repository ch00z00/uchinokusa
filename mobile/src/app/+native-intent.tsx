import { rewriteInvitePath } from '@/features/invitation/inviteLink';

// アプリの外から開かれたリンク（Universal Links / App Links、カスタムスキーム）を、画面に割り当てる前に読み替える。
// 招待リンク https://<招待用ドメイン>/i/<トークン> → 参加の確認 /invite/<トークン>（INV-12）。
// 招待用ドメインでアプリが開くようにするのは、タスク 15.3
export function redirectSystemPath({ path }: { path: string; initial: boolean }): string {
  return rewriteInvitePath(path);
}
