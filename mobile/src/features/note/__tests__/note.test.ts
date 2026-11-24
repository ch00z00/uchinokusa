import { authorOf, canModifyNote, flattenNotes, maxNoteLength, noteLength, validateNote, type Note } from '../note';

const note = (over: Partial<Note> = {}): Note => ({
  id: 'n1',
  plantId: 'p1',
  body: '新芽が出た',
  authorUserId: 'u1',
  authorDisplayName: 'ゆうすけ',
  createdAt: '2026-11-23T01:00:00Z',
  ...over,
});

describe('validateNote', () => {
  it('前後の空白を除いて空なら required', () => {
    expect(validateNote('')).toBe('required');
    expect(validateNote(' \n　')).toBe('required');
    expect(validateNote(' 水をあげた ')).toBeUndefined();
  });

  it('2000文字を超えると too_long。絵文字も1文字と数える', () => {
    expect(validateNote('あ'.repeat(maxNoteLength))).toBeUndefined();
    expect(validateNote('あ'.repeat(maxNoteLength + 1))).toBe('too_long');
    expect(noteLength('🌱🌱')).toBe(2);
    expect(validateNote('🌱'.repeat(maxNoteLength))).toBeUndefined();
  });

  it('文字数は、保存されるとおり前後の空白を除いて数える', () => {
    expect(noteLength('  あい  ')).toBe(2);
  });
});

describe('canModifyNote', () => {
  it('書いた本人とオーナーだけ', () => {
    expect(canModifyNote(note(), 'u1', 'editor')).toBe(true);
    expect(canModifyNote(note(), 'u2', 'editor')).toBe(false);
    expect(canModifyNote(note(), 'u2', 'owner')).toBe(true);
    expect(canModifyNote(note(), 'u1', 'viewer')).toBe(false);
  });

  it('アカウントを削除した人のメモは、オーナーだけ', () => {
    const orphan = note({ authorUserId: undefined });
    expect(canModifyNote(orphan, 'u1', 'editor')).toBe(false);
    expect(canModifyNote(orphan, 'u1', 'owner')).toBe(true);
  });
});

describe('authorOf', () => {
  it('アカウントを削除していれば退会したユーザー', () => {
    expect(authorOf(note())).toEqual({ status: 'member', name: 'ゆうすけ' });
    expect(authorOf(note({ authorUserId: undefined }))).toEqual({ status: 'deleted' });
  });
});

describe('flattenNotes', () => {
  it('ページをつなぎ、ずれて重なったメモは1回だけ出す', () => {
    const pages = [
      { notes: [note({ id: 'a' }), note({ id: 'b' })] },
      { notes: [note({ id: 'b' }), note({ id: 'c' })] },
    ];
    expect(flattenNotes(pages).map((n) => n.id)).toEqual(['a', 'b', 'c']);
  });
});
