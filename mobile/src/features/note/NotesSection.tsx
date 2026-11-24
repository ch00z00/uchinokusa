import { router } from 'expo-router';
import { useEffect, type MutableRefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useMe } from '@/features/account/useMe';
import { can } from '@/features/household/permissions';
import { useTheme } from '@/theme';

import { canModifyNote, flattenNotes } from './note';
import { NoteListItem } from './NoteListItem';
import { useNotes } from './useNotes';

type Props = {
  plantId: string;
  /** 世帯でのロール。メモを書けるのは編集者以上、直せるのは書いた本人とオーナー */
  role: string;
  /** 詳細の画面のスクロールが下の端に近づいたときに呼ぶ関数を入れる（続きの読み込み。HistorySection と同じ） */
  endReachedRef: MutableRefObject<(() => void) | undefined>;
};

/**
 * 植物の詳細の「メモ」（PLANT-13、要件13-1〜13-3）。
 * 上に「メモを書く」（編集者以上。アーカイブ中の植物にも書ける）、その下にメモの一覧（新しい順）
 */
export function NotesSection({ plantId, role, endReachedRef }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const notes = useNotes(plantId);
  const list = flattenNotes(notes.data?.pages);
  const canWrite = can(role, 'edit');
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = notes;

  useEffect(() => {
    endReachedRef.current = () => {
      if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
    };
    return () => {
      endReachedRef.current = undefined;
    };
  }, [endReachedRef, hasNextPage, isFetchingNextPage, fetchNextPage]);

  return (
    <View>
      {canWrite && (
        <ListRow
          title={t('notes.add')}
          accent
          onPress={() => router.push({ pathname: '/plants/[id]/notes/new', params: { id: plantId } })}
        />
      )}
      {notes.isError ? (
        <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
          <AppText color="secondary">{t(errorMessageKey(notes.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void notes.refetch()} />
        </View>
      ) : notes.isPending ? (
        <ActivityIndicator color={theme.color.text.secondary} style={{ padding: theme.space.lg }} />
      ) : list.length === 0 ? (
        <AppText color="secondary" style={{ padding: theme.space.lg }}>
          {t(canWrite ? 'notes.emptyEditor' : 'notes.empty')}
        </AppText>
      ) : (
        <>
          {list.map((note) => (
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
          {isFetchingNextPage && (
            <ActivityIndicator color={theme.color.text.secondary} style={{ padding: theme.space.lg }} />
          )}
        </>
      )}
    </View>
  );
}
