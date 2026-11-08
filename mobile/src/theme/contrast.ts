// WCAG 2.x のコントラスト比。foundations.md の「比」の列を、トークンの値から確かめるために使う
// https://www.w3.org/TR/WCAG22/#dfn-contrast-ratio

function channel(value: number): number {
  const c = value / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

function relativeLuminance(hex: string): number {
  const match = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!match) throw new Error(`#RRGGBB の形ではない色：${hex}`);
  const n = parseInt(match[1], 16);
  const [r, g, b] = [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff].map(channel);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** 2つの色のコントラスト比（1〜21） */
export function contrastRatio(foreground: string, background: string): number {
  const [lighter, darker] = [relativeLuminance(foreground), relativeLuminance(background)].sort((a, b) => b - a);
  return (lighter + 0.05) / (darker + 0.05);
}
