import { filterPlantsByLocation, sortPlantsByArchivedAt, sortPlantsByNextDue, type PlantListEntry } from '../plantList';

const plant = (name: string, extra: Partial<PlantListEntry> = {}): PlantListEntry => ({ id: name, name, ...extra });

describe('sortPlantsByNextDue', () => {
  it('次の予定日が近い順。予定のない植物は最後に、名前順で並べる', () => {
    const sorted = sortPlantsByNextDue([
      plant('ポトス'),
      plant('モンステラ', { nextDueOn: '2026-11-30' }),
      plant('アイビー'),
      plant('サボテン', { nextDueOn: '2026-11-21' }),
    ]);
    expect(sorted.map((p) => p.name)).toEqual(['サボテン', 'モンステラ', 'アイビー', 'ポトス']);
  });

  it('予定日が同じなら名前順', () => {
    const sorted = sortPlantsByNextDue([
      plant('ポトス', { nextDueOn: '2026-11-21' }),
      plant('アイビー', { nextDueOn: '2026-11-21' }),
    ]);
    expect(sorted.map((p) => p.name)).toEqual(['アイビー', 'ポトス']);
  });

  it('元の配列は変えない', () => {
    const plants = [plant('b'), plant('a')];
    sortPlantsByNextDue(plants);
    expect(plants.map((p) => p.name)).toEqual(['b', 'a']);
  });
});

describe('filterPlantsByLocation', () => {
  const plants = [plant('a', { locationId: 'L1' }), plant('b'), plant('c', { locationId: 'L2' })];

  it('all はすべて', () => {
    expect(filterPlantsByLocation(plants, { kind: 'all' })).toHaveLength(3);
  });

  it('unset は置き場所が未設定の植物だけ', () => {
    expect(filterPlantsByLocation(plants, { kind: 'unset' }).map((p) => p.name)).toEqual(['b']);
  });

  it('location はその置き場所の植物だけ', () => {
    expect(filterPlantsByLocation(plants, { kind: 'location', id: 'L2' }).map((p) => p.name)).toEqual(['c']);
  });
});

describe('sortPlantsByArchivedAt', () => {
  const archived = (name: string, archivedAt?: string) => ({ id: name, name, archivedAt });

  it('アーカイブした日時の新しい順。同じなら名前順', () => {
    const sorted = sortPlantsByArchivedAt([
      archived('ポトス', '2026-10-01T09:00:00Z'),
      archived('モンステラ', '2026-11-20T09:00:00Z'),
      archived('アイビー', '2026-10-01T09:00:00Z'),
    ]);
    expect(sorted.map((p) => p.name)).toEqual(['モンステラ', 'アイビー', 'ポトス']);
  });

  it('日時がないもの（来ないはずだが）は最後', () => {
    const sorted = sortPlantsByArchivedAt([archived('ポトス'), archived('アイビー', '2026-10-01T09:00:00Z')]);
    expect(sorted.map((p) => p.name)).toEqual(['アイビー', 'ポトス']);
  });
});
