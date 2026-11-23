import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { api, unwrap } from '@/api';
import { getTodayGrouping, setTodayGrouping } from '@/lib/deviceStorage';
import { toDayString } from '@/lib/formatDate';

import { parseGrouping, type TodayGrouping } from './todayList';

export const todayKeys = {
  /** 世帯の取得の下に置き、タスクの変更（世帯をまとめて取り直す）で取り直す。日付が変わったら別のキーにする */
  list: (householdId: string, date: string) => ['households', householdId, 'today', date] as const,
};

/** 世帯の今日やること（GET /households/{id}/today。端末での今日。閲覧者も見られる。要件9-1） */
export function useToday(householdId: string) {
  const date = toDayString(new Date());
  return useQuery({
    queryKey: todayKeys.list(householdId, date),
    enabled: householdId !== '',
    queryFn: () =>
      unwrap(api.GET('/households/{householdId}/today', { params: { path: { householdId }, query: { date } } })),
  });
}

/** 今日やることの表示（要件9-5）。端末に保存し、次の起動でも同じ表示にする */
export function useTodayGrouping(): [TodayGrouping, (next: TodayGrouping) => void] {
  const [grouping, setGrouping] = useState<TodayGrouping>('date');
  useEffect(() => {
    let active = true;
    getTodayGrouping()
      .then((saved) => {
        if (active) setGrouping(parseGrouping(saved));
      })
      // 読めなければ予定日順のまま
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);
  const change = (next: TodayGrouping) => {
    setGrouping(next);
    void setTodayGrouping(next).catch(() => undefined);
  };
  return [grouping, change];
}
