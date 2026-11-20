import i18n from '@/i18n';

const dayFormat = () => new Intl.DateTimeFormat(i18n.language, { month: 'long', day: 'numeric', weekday: 'short' });

/**
 * 日時を「11月26日(木)」の形で出す（端末のタイムゾーン、アプリの言語）。
 * 相対の表現（「明日」など）を使うかは screen-inventory.md の 6.6 で未決なので、いまは日付だけ
 */
export function formatDate(iso: string): string {
  return dayFormat().format(new Date(iso));
}

/**
 * API の日付（`YYYY-MM-DD`）を、端末のタイムゾーンのその日にする。
 * `new Date('2026-11-26')` は UTC の0時として読まれ、UTC より遅れたタイムゾーンでは前の日になるため
 */
export function parseDay(date: string): Date {
  const [y, m, d] = date.split('-').map(Number);
  return new Date(y ?? 0, (m ?? 1) - 1, d ?? 1);
}

/** API の日付（`YYYY-MM-DD`）を「11月26日(木)」の形で出す */
export function formatDay(date: string): string {
  return dayFormat().format(parseDay(date));
}
