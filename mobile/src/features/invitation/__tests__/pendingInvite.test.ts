import { clearPendingInvite, homeHref, rememberPendingInvite } from '../pendingInvite';

describe('pendingInvite', () => {
  afterEach(clearPendingInvite);

  it('招待を開いていなければ、サインインのあとは「今日」へ', () => {
    expect(homeHref()).toBe('/today');
  });

  it('未サインインで開いた招待は、サインイン（と表示名の設定）のあとに、その参加の確認へ（要件3-4）', () => {
    rememberPendingInvite('abc');
    expect(homeHref()).toBe('/invite/abc');
  });

  it('参加の確認を開いたら忘れる（次のサインインでは「今日」へ）', () => {
    rememberPendingInvite('abc');
    clearPendingInvite();
    expect(homeHref()).toBe('/today');
  });

  it('あとから開いた招待で上書きする', () => {
    rememberPendingInvite('abc');
    rememberPendingInvite('xyz');
    expect(homeHref()).toBe('/invite/xyz');
  });
});
