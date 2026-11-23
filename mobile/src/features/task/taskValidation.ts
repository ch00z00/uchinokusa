import type { components } from '@/api/schema';

import {
  careKinds,
  seasons,
  type CareKind,
  type CareTask,
  type CareTaskDefault,
  type IntervalUnit,
  type Season,
} from './careTask';

// タスクの入力の決まり（api/openapi.yaml の CreateCareTaskRequest・IntervalInput。サーバーと同じ）
export const maxCustomNameLength = 20;
export const maxConditionTextLength = 100;
export const maxNoteLength = 200;
export const maxIntervalValue: Record<IntervalUnit, number> = { day: 365, week: 52, month: 24 };

/**
 * 種類ごとの周期の目安。出典で確かめたものだけを置く（園芸の値を推測で決めない）。
 * 植え替えの24か月は、提案の採用で作るタスクと同じ（docs/decisions/0028）
 */
const intervalPresets: Partial<Record<CareKind, IntervalForm>> = {
  repotting: { value: '24', unit: 'month' },
};

/** 周期の入力。数は入力欄の文字のまま持つ */
export type IntervalForm = { value: string; unit: IntervalUnit };

export type TaskFormInput = {
  /** 空文字は未選択（作成のときは選んでもらう） */
  kind: CareKind | '';
  /** 作業名（「その他」のときだけ使う） */
  customName: string;
  interval: IntervalForm;
  /** 季節ごとの周期（要件8-6）。enabled が false なら使わない */
  seasonal: { enabled: boolean } & Record<Season, IntervalForm>;
  /** 予定日（`YYYY-MM-DD`）。作成では初回の予定日 */
  nextDueOn: string;
  conditionText: string;
  note: string;
};

export type TaskField = Exclude<keyof TaskFormInput, 'seasonal'> | `seasonal.${Season}`;
export type TaskFieldError = 'required' | 'too_long' | 'invalid' | 'past';
export type TaskFormResult =
  | { ok: true; value: TaskFormInput & { kind: CareKind } }
  | { ok: false; errors: Partial<Record<TaskField, TaskFieldError>> };

const emptyInterval: IntervalForm = { value: '', unit: 'day' };

/** 作成の初期値。予定日は今日 */
export function emptyTaskForm(today: string): TaskFormInput {
  return {
    kind: '',
    customName: '',
    interval: emptyInterval,
    seasonal: {
      enabled: false,
      spring: emptyInterval,
      summer: emptyInterval,
      autumn: emptyInterval,
      winter: emptyInterval,
    },
    nextDueOn: today,
    conditionText: '',
    note: '',
  };
}

const length = (s: string) => [...s].length;

/** 全角の数字を半角にし、1以上の整数として読む。読めなければ undefined */
function parseCount(text: string): number | undefined {
  const half = text.trim().replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0));
  return /^\d+$/.test(half) ? Number(half) : undefined;
}

function intervalError(interval: IntervalForm): TaskFieldError | undefined {
  if (interval.value.trim() === '') return 'required';
  const count = parseCount(interval.value);
  if (count === undefined || count < 1 || count > maxIntervalValue[interval.unit]) return 'invalid';
  return undefined;
}

/**
 * 入力を確かめる。today は端末での今日（`YYYY-MM-DD`）。
 * minDueOn を渡すと、それより前の予定日を past にする（作成では今日。渡さなければ確かめない）
 */
export function validateTaskForm(input: TaskFormInput, minDueOn?: string): TaskFormResult {
  const errors: Partial<Record<TaskField, TaskFieldError>> = {};
  const customName = input.customName.trim();
  const conditionText = input.conditionText.trim();
  const note = input.note.trim();

  if (input.kind === '') errors.kind = 'required';
  if (input.kind === 'other') {
    if (customName === '') errors.customName = 'required';
    else if (length(customName) > maxCustomNameLength) errors.customName = 'too_long';
  }
  // 季節ごとに変えるときは、基本の周期の欄を隠して4つの季節だけを入れる（2026-11-22）
  if (input.seasonal.enabled) {
    for (const season of seasons) {
      const error = intervalError(input.seasonal[season]);
      if (error) errors[`seasonal.${season}`] = error;
    }
  } else {
    const base = intervalError(input.interval);
    if (base) errors.interval = base;
  }
  if (input.nextDueOn === '') errors.nextDueOn = 'required';
  // YYYY-MM-DD は文字列の順が日付の順
  else if (minDueOn !== undefined && input.nextDueOn < minDueOn) errors.nextDueOn = 'past';
  if (length(conditionText) > maxConditionTextLength) errors.conditionText = 'too_long';
  if (length(note) > maxNoteLength) errors.note = 'too_long';

  if (input.kind === '' || Object.keys(errors).length > 0) return { ok: false, errors };
  return { ok: true, value: { ...input, kind: input.kind, customName, conditionText, note } };
}

type IntervalInput = components['schemas']['IntervalInput'];

const toInterval = (interval: IntervalForm): IntervalInput => ({
  value: parseCount(interval.value) ?? 0,
  unit: interval.unit,
});

/**
 * 送る基本の周期。季節ごとに変えるときは、4つの季節がすべて入っていて基本の周期は計算に使われないが、
 * API が必須にしているので、春の周期を入れる
 */
const toBaseInterval = (value: TaskFormInput): IntervalInput =>
  toInterval(value.seasonal.enabled ? value.seasonal.spring : value.interval);

const toSeasonal = (seasonal: TaskFormInput['seasonal']): components['schemas']['SeasonalIntervalsInput'] =>
  seasonal.enabled ? Object.fromEntries(seasons.map((s) => [s, toInterval(seasonal[s])])) : {};

/** 作成で送る値。空の任意の項目と、閉じた季節ごとの周期は送らない。作業名は「その他」のときだけ */
export function toCreateTaskRequest(
  value: TaskFormInput & { kind: CareKind },
): components['schemas']['CreateCareTaskRequest'] {
  const seasonal = toSeasonal(value.seasonal);
  return {
    kind: value.kind,
    ...(value.kind === 'other' ? { customName: value.customName } : {}),
    interval: toBaseInterval(value),
    ...(value.seasonal.enabled ? { seasonalIntervals: seasonal } : {}),
    ...(value.conditionText !== '' ? { conditionText: value.conditionText } : {}),
    ...(value.note !== '' ? { note: value.note } : {}),
    nextDueOn: value.nextDueOn,
  };
}

/**
 * 変更で送る値（種類は変えられない）。空の実行条件・メモは空文字で消し、閉じた季節ごとの周期は {} で消す。
 * 予定日は、開いたとき（`initial`）から変えたときだけ送る。送った予定日は手で決めた日になり、
 * 季節の切り替えで計算し直さなくなるため（API の決まり）
 */
export function toUpdateTaskRequest(
  value: TaskFormInput,
  initial: TaskFormInput,
): components['schemas']['UpdateCareTaskRequest'] {
  return {
    ...(value.kind === 'other' ? { customName: value.customName } : {}),
    interval: toBaseInterval(value),
    seasonalIntervals: toSeasonal(value.seasonal),
    conditionText: value.conditionText,
    note: value.note,
    ...(value.nextDueOn !== initial.nextDueOn ? { nextDueOn: value.nextDueOn } : {}),
  };
}

/** 編集で選べる最初の予定日（`YYYY-MM-DD`）。今日と今の予定日の早いほう（期限切れのタスクも、そのまま保存できるように） */
export const editMinDueOn = (today: string, current: string): string =>
  current !== '' && current < today ? current : today;

const intervalToForm = (interval: { value: number; unit: IntervalUnit }): IntervalForm => ({
  value: String(interval.value),
  unit: interval.unit,
});

/** API のタスクを、入力欄の値にする（編集の初期値）。設定のない季節は、基本の周期で埋める */
export function taskToForm(
  task: Pick<
    CareTask,
    'kind' | 'customName' | 'interval' | 'seasonalIntervals' | 'nextDueOn' | 'conditionText' | 'note'
  >,
): TaskFormInput {
  const base = intervalToForm(task.interval);
  const seasonal = Object.fromEntries(
    seasons.map((s) => {
      const interval = task.seasonalIntervals[s];
      return [s, interval ? intervalToForm(interval) : base];
    }),
  ) as Record<Season, IntervalForm>;
  return {
    kind: careKinds.includes(task.kind) ? task.kind : 'other',
    customName: task.customName ?? '',
    interval: base,
    seasonal: { enabled: seasons.some((s) => task.seasonalIntervals[s] !== undefined), ...seasonal },
    nextDueOn: task.nextDueOn ?? '',
    conditionText: task.conditionText ?? '',
    note: task.note ?? '',
  };
}

const sameInterval = (a: IntervalForm | undefined, b: IntervalForm) =>
  a !== undefined && a.value === b.value && a.unit === b.unit;

/**
 * 作業の種類を変える。実行条件と周期は、空か、前の種類のおすすめ・目安のままなら、新しい種類のものに入れ替える
 * （自分で書いた値は消さない）。おすすめの実行条件は API の task-defaults（植物マスタ。要件6-4）
 */
export function changeKind(input: TaskFormInput, kind: CareKind, defaults: readonly CareTaskDefault[]): TaskFormInput {
  const conditionOf = (k: CareKind | '') => defaults.find((d) => d.kind === k)?.conditionText ?? '';
  const prevPreset = input.kind === '' ? undefined : intervalPresets[input.kind];
  const keepCondition = input.conditionText !== '' && input.conditionText !== conditionOf(input.kind);
  const keepInterval = input.interval.value !== '' && !sameInterval(prevPreset, input.interval);
  return {
    ...input,
    kind,
    conditionText: keepCondition ? input.conditionText : conditionOf(kind),
    interval: keepInterval ? input.interval : (intervalPresets[kind] ?? emptyInterval),
  };
}

/** 実行条件が、植物マスタのおすすめのままか（「おすすめの値です」を出す） */
export function isRecommendedCondition(input: TaskFormInput, defaults: readonly CareTaskDefault[]): boolean {
  const recommended = defaults.find((d) => d.kind === input.kind)?.conditionText;
  return recommended !== undefined && recommended !== '' && input.conditionText === recommended;
}
