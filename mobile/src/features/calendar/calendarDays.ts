import type { components } from '@/api/schema';
import type { Note } from '@/features/note/note';
import type { CareRecord } from '@/features/record/careRecord';
import { normalizeCareKind, type CareKind } from '@/features/task/careTask';

export type PlantCalendar = components['schemas']['PlantCalendar'];
export type CalendarPlan = components['schemas']['CalendarPlan'];

/** 日のマスの印の状態（要件14-2）。完了・スキップは記録、予定はタスクの予定日と見込み */
export type MarkerStatus = 'completed' | 'skipped' | 'planned';
export type DayMarker = { kind: CareKind; status: MarkerStatus };
export type CalendarDayInfo = { markers: DayMarker[]; hasNote: boolean };

/** 1つのマスに置ける印の数（1段に2つ）。超えたら最後の枠を「+n」にする。行を低くして、下の日付の詳細が見えるように */
export const markerLimit = 2;

const statusOrder: readonly MarkerStatus[] = ['completed', 'skipped', 'planned'];

/**
 * 月のカレンダーを、日ごとの印にまとめる（要件14-1〜14-3）。
 * 印は完了・スキップ・予定の順。同じ日の同じ種類・状態（追加の記録など）は1つにまとめる
 */
export function calendarDays(calendar: PlantCalendar): Map<string, CalendarDayInfo> {
  const days = new Map<string, CalendarDayInfo>();
  const day = (date: string) => {
    let info = days.get(date);
    if (!info) {
      info = { markers: [], hasNote: false };
      days.set(date, info);
    }
    return info;
  };
  const entries: { date: string; marker: DayMarker }[] = [
    ...calendar.records.map((r) => ({
      date: r.performedOn,
      marker: {
        kind: normalizeCareKind(r.kind),
        status: r.outcome === 'skipped' ? 'skipped' : 'completed',
      } as DayMarker,
    })),
    ...calendar.plans.map((p) => ({
      date: p.date,
      marker: { kind: normalizeCareKind(p.kind), status: 'planned' } as DayMarker,
    })),
  ];
  for (const status of statusOrder) {
    for (const { date, marker } of entries) {
      if (marker.status !== status) continue;
      const info = day(date);
      if (!info.markers.some((m) => m.kind === marker.kind && m.status === marker.status)) info.markers.push(marker);
    }
  }
  for (const { date } of calendar.notes) day(date).hasNote = true;
  return days;
}

/** マスに出す印。入りきらなければ、最後の枠を「+n」にする */
export function visibleMarkers(markers: readonly DayMarker[]): { shown: DayMarker[]; more: number } {
  if (markers.length <= markerLimit) return { shown: [...markers], more: 0 };
  return { shown: markers.slice(0, markerLimit - 1), more: markers.length - (markerLimit - 1) };
}

/** 選んだ日の記録・予定・メモ（PLANT-16、要件14-4） */
export function dayDetail(
  calendar: PlantCalendar,
  date: string,
): { records: CareRecord[]; plans: CalendarPlan[]; notes: Note[] } {
  return {
    records: calendar.records.filter((r) => r.performedOn === date),
    plans: calendar.plans.filter((p) => p.date === date),
    notes: calendar.notes.filter((n) => n.date === date).map((n) => n.note),
  };
}

/** 月（`YYYY-MM`）を前後に動かす（要件14-5） */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number);
  const index = (y ?? 0) * 12 + ((m ?? 1) - 1) + delta;
  return `${Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}`;
}

/** 月を開いたときに選ぶ日。今日がその月なら今日、ほかの月なら1日 */
export function initialSelectedDate(month: string, today: string): string {
  return today.startsWith(month) ? today : `${month}-01`;
}
