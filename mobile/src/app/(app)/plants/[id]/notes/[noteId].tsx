import { useQueryClient, type InfiniteData } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useToast } from '@/components/Toast';
import { flattenNotes, type Note, type NotePage } from '@/features/note/note';
import { NoteEditor } from '@/features/note/NoteEditor';
import { noteKeys, useDeleteNote, useUpdateNote } from '@/features/note/useNotes';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// PLANT-14 メモの編集・削除（モーダル。要件13-3）。メモのタブ（PLANT-13）のメモを押すと開く。書いた本人とオーナー
export default function EditNoteScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id, noteId } = useLocalSearchParams<{ id: string; noteId: string }>();
  const queryClient = useQueryClient();
  // メモ1件を取る API はないので、メモの一覧の取得の結果から探す。保存・削除で一覧を取り直しても、開いたときのメモを使い続ける
  const [note] = useState(() =>
    flattenNotes(queryClient.getQueryData<InfiniteData<NotePage>>(noteKeys.list(id))?.pages).find(
      (n) => n.id === noteId,
    ),
  );
  if (!note) {
    return (
      <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
        <AppText color="secondary">{t('notes.missing')}</AppText>
      </ScrollView>
    );
  }
  return <EditNote note={note} />;
}

function EditNote({ note }: { note: Note }) {
  const { t } = useTranslation();
  const toast = useToast();
  const update = useUpdateNote(note.plantId, note.id);
  const remove = useDeleteNote(note.plantId, note.id);

  const deleteNote = async () => {
    const ok = await confirm({
      title: t('notes.delete.confirmTitle'),
      message: t('notes.delete.confirmMessage'),
      confirmLabel: t('notes.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    try {
      await remove.mutateAsync();
    } catch {
      return;
    }
    toast.show(t('notes.delete.done'));
    router.back();
  };

  return (
    <NoteEditor
      initialBody={note.body}
      pending={update.isPending}
      error={update.error}
      onSave={async (body) => {
        // 本文を変えていなければ送らない（サーバーは変わったときだけ「編集済み」にするが、通信を省く）
        if (body === note.body) return true;
        try {
          await update.mutateAsync(body);
          return true;
        } catch {
          return false;
        }
      }}
      onDelete={() => void deleteNote()}
      deleting={remove.isPending}
      deleteError={remove.error}
    />
  );
}
