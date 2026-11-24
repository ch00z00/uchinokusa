// デザイントークン。値の正は docs/design/foundations.md（Figma とコードの両方の元）。
// 値を変えるときは、先に foundations.md を直してから、ここに写す。
//
// 名前の読み替え（.kiro/steering/design.md）：foundations.md の `color/care/watering` ⇔ `theme.color.care.watering`
// - 子を持つトークンは、自身の値を `default` に置く：`color/accent` ⇔ `color.accent.default`、`color/accent/subtle` ⇔ `color.accent.subtle`
// - ハイフンはキャメルケースにする：`color/on-accent` ⇔ `color.onAccent`、`text/large-title` ⇔ `text.largeTitle`

// 1. 色（foundations.md の 1章）。ライトとダークの2つの値を持つ
const light = {
  bg: {
    base: '#F6F1EA',
    surface: '#FFFDF9',
    subtle: '#EFE7DC',
  },
  border: {
    subtle: '#E3D9CC',
  },
  text: {
    primary: '#2B2420',
    secondary: '#6B5E55',
    tertiary: '#8F8174',
  },
  accent: {
    default: '#AE4F2C',
    subtle: '#F3DED3',
  },
  onAccent: '#FFFFFF',
  danger: '#A8231C',
  success: '#3F7449',
  care: {
    watering: '#3A739C',
    fertilizing: '#86650F',
    repotting: '#87553A',
    pruning: '#4A7754',
    other: '#73668A',
  },
  suggestion: {
    avoid: '#646E78',
    conditional: '#4F5F8A',
  },
  // QR コードは、ダークでもライトと同じ値（明るい地に暗い模様。反転した QR を読めないカメラがある）
  qr: {
    foreground: '#2B2420',
    background: '#FFFDF9',
  },
};

export type ColorTokens = typeof light & {
  status: { overdue: string };
  suggestion: { do: string };
};

const dark: typeof light = {
  bg: {
    base: '#1C1815',
    surface: '#262019',
    subtle: '#2F2821',
  },
  border: {
    subtle: '#3D342C',
  },
  text: {
    primary: '#F3ECE4',
    secondary: '#BFB2A6',
    tertiary: '#958879',
  },
  accent: {
    default: '#E8875F',
    subtle: '#3E2A20',
  },
  onAccent: '#1C1815',
  danger: '#F2786F',
  success: '#86BE91',
  care: {
    watering: '#76AEDA',
    fertilizing: '#D6AE4C',
    repotting: '#CC9470',
    pruning: '#86B791',
    other: '#AFA2CA',
  },
  suggestion: {
    avoid: '#A3ADB7',
    conditional: '#97A6D0',
  },
  qr: light.qr,
};

// 別のトークンと同じ値のもの（`= color/accent`）は、参照でつなぐ
function withAliases(base: typeof light): ColorTokens {
  return {
    ...base,
    status: { overdue: base.accent.default },
    suggestion: { ...base.suggestion, do: base.accent.default },
  };
}

export const colors = {
  light: withAliases(light),
  dark: withAliases(dark),
} as const;

// 2. 文字（foundations.md の 2章）。HIG の文字スタイルの「標準」の大きさ（pt）。
// React Native の Text は、端末の文字の大きさの設定（Dynamic Type）に合わせて自動で拡大する（allowFontScaling を false にしない）
export const text = {
  largeTitle: { fontSize: 34, lineHeight: 41, fontWeight: '700' },
  title2: { fontSize: 22, lineHeight: 28, fontWeight: '700' },
  title3: { fontSize: 20, lineHeight: 25, fontWeight: '600' },
  headline: { fontSize: 17, lineHeight: 22, fontWeight: '600' },
  body: { fontSize: 17, lineHeight: 22, fontWeight: '400' },
  callout: { fontSize: 16, lineHeight: 21, fontWeight: '400' },
  subheadline: { fontSize: 15, lineHeight: 20, fontWeight: '400' },
  footnote: { fontSize: 13, lineHeight: 18, fontWeight: '400' },
  caption: { fontSize: 12, lineHeight: 16, fontWeight: '400' },
} as const;

export type TextVariant = keyof typeof text;

// 3. 余白・角丸・大きさ（foundations.md の 3章）。単位は pt
export const space = {
  '2xs': 2,
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  '2xl': 32,
} as const;

export const radius = {
  sm: 6,
  md: 10,
  lg: 14,
  full: 999,
} as const;

export const size = {
  tapMin: 44,
  icon: {
    xs: 11,
    sm: 16,
    md: 20,
    lg: 28,
  },
} as const;

// shadow/card は「なし」（border/subtle の 1pt の枠で区切る）なので、トークンを作らない
export const borderWidth = {
  hairline: 1,
  thick: 2,
  /** 行の左の帯（期限切れの項目など、行ごと目立たせる） */
  emphasis: 4,
} as const;
