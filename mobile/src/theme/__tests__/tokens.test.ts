import { contrastRatio } from '../contrast';
import { colors } from '../tokens';

describe('contrastRatio', () => {
  it('白と黒は 21、同じ色は 1', () => {
    expect(contrastRatio('#FFFFFF', '#000000')).toBeCloseTo(21);
    expect(contrastRatio('#AE4F2C', '#AE4F2C')).toBeCloseTo(1);
  });

  it('前景と背景を入れ替えても同じ', () => {
    expect(contrastRatio('#2B2420', '#F6F1EA')).toBeCloseTo(contrastRatio('#F6F1EA', '#2B2420'));
  });
});

// foundations.md の 1章の「比（ライト / ダーク）」の列。値を変えたら、ここと foundations.md の両方を直す
describe.each(['light', 'dark'] as const)('%s の色', (scheme) => {
  const c = colors[scheme];
  const onBase = (color: string) => contrastRatio(color, c.bg.base);

  it('本文の文字（primary・secondary）は、背景に対して 4.5 以上', () => {
    expect(onBase(c.text.primary)).toBeGreaterThanOrEqual(4.5);
    expect(onBase(c.text.secondary)).toBeGreaterThanOrEqual(4.5);
  });

  it('アクセント・危険・成功は、背景に対して 4.5 以上（文字やアイコンに使うため）', () => {
    expect(onBase(c.accent.default)).toBeGreaterThanOrEqual(4.5);
    expect(onBase(c.danger)).toBeGreaterThanOrEqual(4.5);
    expect(onBase(c.success)).toBeGreaterThanOrEqual(4.5);
  });

  it('アクセントの上の文字は、アクセントに対して 4.5 以上', () => {
    expect(contrastRatio(c.onAccent, c.accent.default)).toBeGreaterThanOrEqual(4.5);
  });

  it.each(['watering', 'fertilizing', 'repotting', 'pruning', 'other'] as const)(
    '作業の種類の色（%s）は、背景に対して 4.5 以上',
    (kind) => {
      expect(onBase(c.care[kind])).toBeGreaterThanOrEqual(4.5);
    },
  );

  it('提案の色は、背景に対して 4.5 以上', () => {
    expect(onBase(c.suggestion.avoid)).toBeGreaterThanOrEqual(4.5);
    expect(onBase(c.suggestion.conditional)).toBeGreaterThanOrEqual(4.5);
  });

  it('期限切れ・おすすめは、アクセントと同じ色', () => {
    expect(c.status.overdue).toBe(c.accent.default);
    expect(c.suggestion.do).toBe(c.accent.default);
  });
});
