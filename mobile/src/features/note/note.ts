import type { components } from '@/api/schema';
import { normalizeRole } from '@/features/household/permissions';

export type Note = components['schemas']['Note'];
export type NotePage = components['schemas']['NotePage'];

/** メモの本文の上限（要件13-4。API の決まり） */
export const maxNoteLength = 2000;

/** 本文の文字数。サーバーと同じく、前後の空白を除いて、絵文字なども1文字と数える */
export function noteLength(body: string): number {
  return [...body.trim()].length;
}

/** 本文の確認（要件13-4）。空（前後の空白だけを含む）と、2000文字を超えるものは保存しない */
export function validateNote(body: string): 'required' | 'too_long' | undefined {
  const length = noteLength(body);
  if (length === 0) return 'required';
  if (length > maxNoteLength) return 'too_long';
  return undefined;
}

/** メモを編集・削除できるか（要件13-3）。書いた本人とオーナーだけ。閲覧者は本人でもできない */
export function canModifyNote(note: Note, myUserId: string | undefined, role: string): boolean {
  const r = normalizeRole(role);
  if (r === 'owner') return true;
  if (r === 'viewer') return false;
  return note.authorUserId !== undefined && note.authorUserId === myUserId;
}

/** 書いた人。アカウントを削除した人は名前を出さず「退会したユーザー」にする */
export function authorOf(note: Note): { status: 'member'; name: string } | { status: 'deleted' } {
  return note.authorUserId === undefined ? { status: 'deleted' } : { status: 'member', name: note.authorDisplayName };
}

/** メモのページをつなぐ。続きを読み込むあいだに新しいメモが入ってずれても、同じメモを2回出さない */
export function flattenNotes(pages: readonly Pick<NotePage, 'notes'>[] | undefined): Note[] {
  const seen = new Set<string>();
  const notes: Note[] = [];
  for (const page of pages ?? []) {
    for (const n of page.notes) {
      if (seen.has(n.id)) continue;
      seen.add(n.id);
      notes.push(n);
    }
  }
  return notes;
}
