import { leaveDecision } from '../leave';

describe('leaveDecision', () => {
  it.each(['editor', 'viewer'])('%s は退出できる', (role) => {
    expect(leaveDecision(role, 3)).toBe('allowed');
  });

  it('オーナーで、ほかにメンバーがいれば、先にオーナー権限を移す（要件2-8）', () => {
    expect(leaveDecision('owner', 2)).toBe('transferRequired');
  });

  it('オーナーで、自分だけなら、退出ではなく世帯を削除する', () => {
    expect(leaveDecision('owner', 1)).toBe('deleteRequired');
  });
});
