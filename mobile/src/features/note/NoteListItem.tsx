import { useTranslation } from 'react-i18next';

import { ListRow } from '@/components/ListRow';
import { formatDate, formatTime } from '@/lib/formatDate';

import { authorOf, type Note } from './note';

type Props = {
  note: Note;
  /** 押すとメモの編集（PLANT-14。書いた本人とオーナー）。渡さなければ押せない */
  onPress?: () => void;
};

/**
 * メモ（foundations.md の 5章の NoteListItem。PLANT-13、要件13-2・13-3）。
 * 本文を全文、その下に作成日時・作成者。編集したメモは「編集済み」を足す
 */
export function NoteListItem({ note, onPress }: Props) {
  const { t } = useTranslation();
  const author = authorOf(note);
  const meta = [
    `${formatDate(note.createdAt)} ${formatTime(note.createdAt)}`,
    author.status === 'member' ? author.name : t('history.deletedUser'),
    note.editedAt ? t('notes.edited') : undefined,
  ]
    .filter(Boolean)
    .join(t('tasks.list.separator'));
  return <ListRow title={note.body} subtitle={meta} navigates={onPress !== undefined} onPress={onPress} />;
}
