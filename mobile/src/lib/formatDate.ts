import i18n from '@/i18n';

/**
 * 日時を「11月26日(木)」の形で出す（端末のタイムゾーン、アプリの言語）。
 * 相対の表現（「明日」など）を使うかは screen-inventory.md の 6.6 で未決なので、いまは日付だけ
 */
export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat(i18n.language, { month: 'long', day: 'numeric', weekday: 'short' }).format(
    new Date(iso),
  );
}
