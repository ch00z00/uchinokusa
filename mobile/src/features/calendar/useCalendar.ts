import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { plantKeys } from '@/features/plant/usePlants';

export const calendarKeys = {
  /** 植物のカレンダーすべて（月ごと）。植物の詳細のキーの下に置き、記録・メモ・タスクの変更でいっしょに取り直す */
  all: (plantId: string) => [...plantKeys.detail(plantId), 'calendar'] as const,
  month: (plantId: string, month: string) => [...calendarKeys.all(plantId), month] as const,
};

/** 植物の1か月のカレンダー（GET /plants/{id}/calendar?month=。閲覧者も見られる。要件14-1〜14-5） */
export function useCalendar(plantId: string, month: string) {
  return useQuery({
    queryKey: calendarKeys.month(plantId, month),
    queryFn: () => unwrap(api.GET('/plants/{plantId}/calendar', { params: { path: { plantId }, query: { month } } })),
    // 月を移ったあいだも、前の月を出したままにする（マスがちらつかないように）
    placeholderData: keepPreviousData,
  });
}
