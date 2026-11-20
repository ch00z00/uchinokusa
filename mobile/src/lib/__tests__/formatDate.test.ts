import { parseDay } from '../formatDate';

describe('parseDay', () => {
  it('YYYY-MM-DD を、端末のタイムゾーンのその日の0時にする（UTC として読んで日がずれないように）', () => {
    const d = parseDay('2026-11-26');
    expect([d.getFullYear(), d.getMonth() + 1, d.getDate(), d.getHours()]).toEqual([2026, 11, 26, 0]);
  });
});
