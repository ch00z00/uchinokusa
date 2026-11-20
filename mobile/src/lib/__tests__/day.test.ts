import { toDayString } from '../formatDate';

describe('toDayString', () => {
  it('端末のタイムゾーンでの日付を YYYY-MM-DD にする', () => {
    expect(toDayString(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05');
  });
});
