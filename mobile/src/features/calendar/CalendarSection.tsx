import { router } from 'expo-router';
import { SymbolView } from 'expo-symbols';
import { createContext, useContext, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { Calendar, LocaleConfig, type DateData } from 'react-native-calendars';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { CalendarDayMarker, markerSize } from '@/components/CalendarDayMarker';
import { CareKindIcon } from '@/components/CareKindIcon';
import { ListRow } from '@/components/ListRow';
import { useMe } from '@/features/account/useMe';
import { canModifyNote } from '@/features/note/note';
import { NoteListItem } from '@/features/note/NoteListItem';
import { RecordListItem } from '@/features/record/RecordListItem';
import { normalizeCareKind } from '@/features/task/careTask';
import { taskTitle } from '@/features/task/taskText';
import i18n from '@/i18n';
import { formatDay, toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

import { calendarDays, dayDetail, initialSelectedDate, visibleMarkers, type CalendarDayInfo } from './calendarDays';
import { useCalendar } from './useCalendar';

// カレンダーの月・曜日の名前を、アプリの言語で作る（react-native-calendars の言語の設定）
const locale = i18n.language;
LocaleConfig.locales[locale] = {
  monthNames: Array.from({ length: 12 }, (_, m) =>
    new Intl.DateTimeFormat(locale, { month: 'long' }).format(new Date(2026, m, 1)),
  ),
  monthNamesShort: Array.from({ length: 12 }, (_, m) =>
    new Intl.DateTimeFormat(locale, { month: 'short' }).format(new Date(2026, m, 1)),
  ),
  // 2026-11-01 は日曜日
  dayNames: Array.from({ length: 7 }, (_, d) =>
    new Intl.DateTimeFormat(locale, { weekday: 'long' }).format(new Date(2026, 10, 1 + d)),
  ),
  dayNamesShort: Array.from({ length: 7 }, (_, d) =>
    new Intl.DateTimeFormat(locale, { weekday: 'short' }).format(new Date(2026, 10, 1 + d)),
  ),
  today: '',
};
LocaleConfig.defaultLocale = locale;

/** 日のマスに渡す、月の印と選んだ日（dayComponent には任意の値を渡せないので、コンテキストで渡す） */
const DayContext = createContext<{
  days: Map<string, CalendarDayInfo>;
  selected: string;
  today: string;
  onSelect: (date: string) => void;
}>({ days: new Map(), selected: '', today: '', onSelect: () => {} });

type Props = {
  plantId: string;
  role: string;
  /** 記録の編集（PLANT-12）ができるか。編集者以上で、アーカイブ中でない植物 */
  canEditRecords: boolean;
};

/**
 * 植物の詳細の「カレンダー」（PLANT-15・PLANT-16、要件14-1〜14-5）。
 * 月のカレンダーに、完了・スキップ・予定とメモの日を示し、選んだ日の記録・予定・メモをカレンダーの下に出す
 */
export function CalendarSection({ plantId, role, canEditRecords }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const today = toDayString(new Date());
  const [month, setMonth] = useState(today.slice(0, 7));
  const [selected, setSelected] = useState(today);
  const calendar = useCalendar(plantId, month);
  const days = useMemo(() => (calendar.data ? calendarDays(calendar.data) : new Map()), [calendar.data]);
  const context = useMemo(() => ({ days, selected, today, onSelect: setSelected }), [days, selected, today]);

  const changeMonth = (date: DateData) => {
    const next = date.dateString.slice(0, 7);
    setMonth(next);
    setSelected(initialSelectedDate(next, today));
  };

  return (
    <View style={{ gap: theme.space.lg }}>
      <DayContext.Provider value={context}>
        <Calendar
          current={`${month}-01`}
          onMonthChange={changeMonth}
          dayComponent={CalendarDay}
          monthFormat={t('calendar.monthFormat')}
          enableSwipeMonths
          hideExtraDays
          theme={{
            calendarBackground: theme.color.bg.base,
            monthTextColor: theme.color.text.primary,
            textMonthFontSize: theme.text.headline.fontSize,
            textMonthFontWeight: theme.text.headline.fontWeight,
            textSectionTitleColor: theme.color.text.secondary,
            textDayHeaderFontSize: theme.text.footnote.fontSize,
            arrowColor: theme.color.accent.default,
          }}
        />
      </DayContext.Provider>
      <Legend />
      {calendar.isError ? (
        <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
          <AppText color="secondary">{t(errorMessageKey(calendar.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void calendar.refetch()} />
        </View>
      ) : !calendar.data ? (
        <ActivityIndicator color={theme.color.text.secondary} />
      ) : (
        <DayDetail
          date={selected}
          detail={dayDetail(calendar.data, selected)}
          plantId={plantId}
          role={role}
          canEditRecords={canEditRecords}
        />
      )}
    </View>
  );
}

/** 日のマス。日付の数字と、メモの印、作業の印（1段に2つまで）。選んだ日は数字をアクセントの円で囲む */
function CalendarDay({ date, state }: { date?: DateData; state?: string }) {
  const theme = useTheme();
  const { t } = useTranslation();
  const { days, selected, today, onSelect } = useContext(DayContext);
  if (!date) return <View />;
  const info = days.get(date.dateString);
  const { shown, more } = visibleMarkers(info?.markers ?? []);
  const isSelected = date.dateString === selected;
  const isToday = date.dateString === today;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ selected: isSelected, disabled: state === 'disabled' }}
      accessibilityLabel={[
        formatDay(date.dateString),
        info?.markers.length ? t('calendar.a11y.markers', { count: info.markers.length }) : undefined,
        info?.hasNote ? t('calendar.legend.note') : undefined,
      ]
        .filter(Boolean)
        .join('、')}
      onPress={() => onSelect(date.dateString)}
      style={[styles.day, { minHeight: theme.size.icon.lg + theme.space.xs + markerSize, gap: theme.space.xs }]}
    >
      <View style={styles.numberLine}>
        <View
          style={[
            styles.number,
            {
              width: theme.size.icon.lg,
              height: theme.size.icon.lg,
              borderRadius: theme.radius.full,
              backgroundColor: isSelected ? theme.color.accent.default : 'transparent',
            },
          ]}
        >
          <AppText
            variant={isToday ? 'headline' : 'callout'}
            style={{
              color: isSelected
                ? theme.color.onAccent
                : isToday
                  ? theme.color.accent.default
                  : theme.color.text.primary,
            }}
          >
            {date.day}
          </AppText>
        </View>
        {info?.hasNote && (
          <SymbolView
            name={{ ios: 'square.and.pencil', android: 'edit_note' }}
            size={theme.size.icon.xs}
            tintColor={theme.color.text.secondary}
            style={[styles.note, { right: -theme.space.md, width: theme.size.icon.xs, height: theme.size.icon.xs }]}
          />
        )}
      </View>
      <View style={[styles.markers, { gap: theme.space.xs }]}>
        {shown.map((m) => (
          <CalendarDayMarker key={`${m.kind}-${m.status}`} kind={m.kind} status={m.status} />
        ))}
        {more > 0 && (
          <AppText variant="footnote" color="secondary">
            {t('calendar.more', { count: more })}
          </AppText>
        )}
      </View>
    </Pressable>
  );
}

/** 凡例（要件14-2）。状態の形と、メモの印 */
function Legend() {
  const { t } = useTranslation();
  const theme = useTheme();
  const items = [
    { key: 'completed', node: <CalendarDayMarker kind="watering" status="completed" /> },
    { key: 'planned', node: <CalendarDayMarker kind="watering" status="planned" /> },
    { key: 'skipped', node: <CalendarDayMarker kind="watering" status="skipped" /> },
  ] as const;
  return (
    <View style={[styles.legend, { paddingHorizontal: theme.space.lg, gap: theme.space.lg }]}>
      {items.map((item) => (
        <View key={item.key} style={[styles.legendItem, { gap: theme.space.xs }]}>
          {item.node}
          <AppText variant="footnote" color="secondary">
            {t(`calendar.legend.${item.key}`)}
          </AppText>
        </View>
      ))}
      <View style={[styles.legendItem, { gap: theme.space.xs }]}>
        <SymbolView
          name={{ ios: 'square.and.pencil', android: 'edit_note' }}
          size={theme.size.icon.xs}
          tintColor={theme.color.text.secondary}
          style={{ width: theme.size.icon.xs, height: theme.size.icon.xs }}
        />
        <AppText variant="footnote" color="secondary">
          {t('calendar.legend.note')}
        </AppText>
      </View>
    </View>
  );
}

/** 選んだ日の記録・予定・メモ（PLANT-16、要件14-4） */
function DayDetail({
  date,
  detail,
  plantId,
  role,
  canEditRecords,
}: {
  date: string;
  detail: ReturnType<typeof dayDetail>;
  plantId: string;
  role: string;
  canEditRecords: boolean;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const empty = detail.records.length === 0 && detail.plans.length === 0 && detail.notes.length === 0;
  return (
    <View style={{ gap: theme.space.sm }}>
      <AppText variant="headline" style={{ paddingHorizontal: theme.space.lg }}>
        {formatDay(date)}
      </AppText>
      {empty ? (
        <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
          {t('calendar.dayEmpty')}
        </AppText>
      ) : (
        <View>
          {detail.records.map((record) => (
            <RecordListItem
              key={record.id}
              record={record}
              onPress={
                canEditRecords
                  ? () =>
                      router.push({
                        pathname: '/plants/[id]/records/[recordId]',
                        params: { id: plantId, recordId: record.id },
                      })
                  : undefined
              }
            />
          ))}
          {detail.plans.map((plan) => (
            <ListRow
              key={`${plan.taskId}-${plan.date}`}
              leading={<CareKindIcon kind={normalizeCareKind(plan.kind)} size="lg" />}
              title={taskTitle(plan, t)}
              subtitle={t(plan.projected ? 'calendar.projected' : 'calendar.planned')}
            />
          ))}
          {detail.notes.map((note) => (
            <NoteListItem
              key={note.id}
              note={note}
              onPress={
                canModifyNote(note, me.data?.id, role)
                  ? () =>
                      router.push({ pathname: '/plants/[id]/notes/[noteId]', params: { id: plantId, noteId: note.id } })
                  : undefined
              }
            />
          ))}
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  day: { alignItems: 'center', alignSelf: 'stretch' },
  numberLine: { position: 'relative' },
  number: { alignItems: 'center', justifyContent: 'center' },
  note: { position: 'absolute', top: 0 },
  markers: { flexDirection: 'row', justifyContent: 'center', alignItems: 'center' },
  legend: { flexDirection: 'row', flexWrap: 'wrap' },
  legendItem: { flexDirection: 'row', alignItems: 'center' },
});
