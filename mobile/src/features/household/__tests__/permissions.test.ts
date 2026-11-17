import { can, normalizeRole, type Permission } from '../permissions';

describe('normalizeRole', () => {
  it.each(['owner', 'editor', 'viewer'] as const)('%s はそのまま', (role) => {
    expect(normalizeRole(role)).toBe(role);
  });

  it('知らないロールは閲覧者として扱う（API の Role の説明のとおり）', () => {
    expect(normalizeRole('admin')).toBe('viewer');
  });
});

describe('can', () => {
  // screen-inventory.md の 6.1 の表
  const table: [Permission, boolean, boolean, boolean][] = [
    // 操作, オーナー, 編集者, 閲覧者
    ['record', true, true, false],
    ['edit', true, true, false],
    ['manageHousehold', true, false, false],
  ];

  it.each(table)('%s：オーナー %s・編集者 %s・閲覧者 %s', (permission, owner, editor, viewer) => {
    expect(can('owner', permission)).toBe(owner);
    expect(can('editor', permission)).toBe(editor);
    expect(can('viewer', permission)).toBe(viewer);
  });

  it('知らないロールは、閲覧者と同じく何もできない', () => {
    expect(can('admin', 'record')).toBe(false);
    expect(can('admin', 'edit')).toBe(false);
  });
});
