import { resolveSelectedHouseholdId } from '../selection';

const households = [{ id: 'home' }, { id: 'parents' }];

describe('resolveSelectedHouseholdId', () => {
  it('保存した世帯に今も所属していれば、その世帯', () => {
    expect(resolveSelectedHouseholdId(households, 'parents')).toBe('parents');
  });

  it('保存していなければ、最初の世帯（作成の古い順の先頭）', () => {
    expect(resolveSelectedHouseholdId(households, null)).toBe('home');
  });

  it('保存した世帯から外れた（退出・削除・外された）なら、最初の世帯', () => {
    expect(resolveSelectedHouseholdId(households, 'deleted')).toBe('home');
  });

  it('どの世帯にも所属していなければ null（はじめての選択を出す）', () => {
    expect(resolveSelectedHouseholdId([], 'home')).toBeNull();
  });
});
