import type { components } from '@/api/schema';

export type SkipReason = components['schemas']['SkipReason'];
export const skipReasons: readonly SkipReason[] = ['soil_wet', 'bad_weather', 'plant_weak', 'other'];

const maxReasonTextLength = 200;

/** スキップの入力（TODAY-03、要件10-6〜10-8） */
export type SkipForm = {
  /** 空なら理由なし（任意） */
  reason: SkipReason | '';
  /** 「その他」のときの自由入力 */
  reasonText: string;
  /** 次回の確認日（`YYYY-MM-DD`）。初期値は明日 */
  recheckOn: string;
};

export type SkipField = 'reasonText' | 'recheckOn';
export type SkipFieldError = 'too_long' | 'past';

export const emptySkipForm = (tomorrow: string): SkipForm => ({ reason: '', reasonText: '', recheckOn: tomorrow });

/** 入力の誤り。自由入力は200文字まで、確認日は明日以降（サーバーと同じ決まり） */
export function validateSkip(form: SkipForm, tomorrow: string): Partial<Record<SkipField, SkipFieldError>> {
  const errors: Partial<Record<SkipField, SkipFieldError>> = {};
  if (form.reason === 'other' && [...form.reasonText.trim()].length > maxReasonTextLength) {
    errors.reasonText = 'too_long';
  }
  // YYYY-MM-DD は文字列の順が日付の順
  if (form.recheckOn < tomorrow) errors.recheckOn = 'past';
  return errors;
}

/** 送る値。理由がなければ送らない。自由入力は「その他」で、空でないときだけ送る */
export function toSkipRequest(form: SkipForm, cycleSeq: number): components['schemas']['SkipCareTaskRequest'] {
  const text = form.reasonText.trim();
  return {
    cycleSeq,
    ...(form.reason !== '' ? { reason: form.reason } : {}),
    ...(form.reason === 'other' && text !== '' ? { reasonText: text } : {}),
    recheckOn: form.recheckOn,
  };
}
