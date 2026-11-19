import { inviteTokenFromScan, rewriteInvitePath } from '../inviteLink';

describe('rewriteInvitePath', () => {
  // 招待リンク https://<招待用ドメイン>/i/<トークン> で開いたとき、アプリの参加の確認（/invite/<トークン>）へ
  it('/i/<トークン> を /invite/<トークン> にする', () => {
    expect(rewriteInvitePath('/i/abc_DEF-123')).toBe('/invite/abc_DEF-123');
  });

  it('URL 全体が渡されても、パスで判定する', () => {
    expect(rewriteInvitePath('https://invite.example.com/i/abc')).toBe('/invite/abc');
  });

  it('ほかのパスはそのまま', () => {
    expect(rewriteInvitePath('/today')).toBe('/today');
    expect(rewriteInvitePath('/i/')).toBe('/i/');
    expect(rewriteInvitePath('/i/abc/extra')).toBe('/i/abc/extra');
  });
});

describe('inviteTokenFromScan', () => {
  it('招待リンクの QR からトークンを取り出す', () => {
    expect(inviteTokenFromScan('https://invite.example.com/i/abc_DEF-123')).toBe('abc_DEF-123');
  });

  it('ローカルの開発用の http のリンクも受け付ける', () => {
    expect(inviteTokenFromScan('http://localhost:8080/i/abc')).toBe('abc');
  });

  it('前後の空白は無視する', () => {
    expect(inviteTokenFromScan(' https://invite.example.com/i/abc\n')).toBe('abc');
  });

  it.each([
    'https://example.com/',
    'https://example.com/i/',
    'https://example.com/i/abc/extra',
    'https://example.com/i/ab%20c',
    'ftp://example.com/i/abc',
    'こんにちは',
    '',
  ])('招待ではない QR は null（%s）', (data) => {
    expect(inviteTokenFromScan(data)).toBeNull();
  });
});
