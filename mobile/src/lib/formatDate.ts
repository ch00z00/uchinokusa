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

/** 端末のタイムゾーンでの日付を、API の形（`YYYY-MM-DD`）にする */
export function toDayString(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** API の日付（`YYYY-MM-DD`）を「2027年4月1日(木)」の形で出す（年をまたぐ予定の提案など） */
export function formatDayWithYear(date: string): string {
  return new Intl.DateTimeFormat(i18n.language, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'short',
  }).format(parseDay(date));
}

/** 日時を「10:15」の形で出す（端末のタイムゾーン、アプリの言語） */
export function formatTime(iso: string): string {
  return new Intl.DateTimeFormat(i18n.language, { hour: '2-digit', minute: '2-digit' }).format(new Date(iso));
}
