import { useLocalSearchParams } from 'expo-router';

import { NoteEditor } from '@/features/note/NoteEditor';
import { useCreateNote } from '@/features/note/useNotes';

// PLANT-14 メモの作成（モーダル。要件13-1）。メモのタブ（PLANT-13）の「メモを書く」から開く。編集者以上
export default function NewNoteScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const create = useCreateNote(id);
  return (
    <NoteEditor
      initialBody=""
      pending={create.isPending}
      error={create.error}
      onSave={async (body) => {
        try {
          await create.mutateAsync(body);
          return true;
        } catch {
          return false;
        }
      }}
    />
  );
}
