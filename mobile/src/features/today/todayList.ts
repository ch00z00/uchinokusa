import type { components } from '@/api/schema';
import { careKinds, normalizeCareKind } from '@/features/task/careTask';

export type TodayList = components['schemas']['TodayList'];
export type TodayItem = components['schemas']['TodayItem'];
type Suggestion = components['schemas']['Suggestion'];

/** 今日やることの表示（要件9-5）。予定日順・置き場所ごと・作業の種類ごと */
export const todayGroupings = ['date', 'location', 'kind'] as const;
export type TodayGrouping = (typeof todayGroupings)[number];

/** 端末に保存した表示を読む。知らない値や未保存は予定日順 */
export const parseGrouping = (value: string | null): TodayGrouping =>
  todayGroupings.includes(value as TodayGrouping) ? (value as TodayGrouping) : 'date';

export type TodaySection = { key: string; title?: string; items: TodayItem[] };

/**
 * 今日やることを、表示に合わせて見出しごとに分ける。見出しの中は API の順（予定日の古い順）のまま。
 * 置き場所ごとは置き場所名の順で、未設定（`locationUnset` を見出しにする）は最後。
 * 作業の種類ごとは水やり・肥料・植え替え・剪定・その他の順で、見出しはアプリが種類の名前にする（key が種類）
 */
export function groupTodayItems(items: TodayItem[], grouping: TodayGrouping, locationUnset: string): TodaySection[] {
  if (grouping === 'date') return [{ key: 'all', items }];
  if (grouping === 'kind') {
    return careKinds
      .map((kind) => ({ key: kind, items: items.filter((i) => normalizeCareKind(i.task.kind) === kind) }))
      .filter((s) => s.items.length > 0);
  }
  const byLocation = new Map<string, TodaySection>();
  for (const i of items) {
    const key = i.plant.locationId ?? 'unset';
    const section = byLocation.get(key) ?? {
      key,
      title: i.plant.locationId ? (i.plant.locationName ?? '') : locationUnset,
      items: [],
    };
    section.items.push(i);
    byLocation.set(key, section);
  }
  const named = [...byLocation.values()].filter((s) => s.key !== 'unset');
  named.sort((a, b) => (a.title ?? '').localeCompare(b.title ?? '', 'ja'));
  const unset = byLocation.get('unset');
  return unset ? [...named, unset] : named;
}

/**
 * タスクのカードに出す提案（要件9-4・19-4）。「やらない方がよい」を「条件つき」より先に選ぶ。
 * 「おすすめ」や知らない種類は、今日の一覧には出さない
 */
export function todaySuggestion(item: TodayItem): Suggestion | undefined {
  return item.suggestions.find((s) => s.kind === 'avoid') ?? item.suggestions.find((s) => s.kind === 'conditional');
}
