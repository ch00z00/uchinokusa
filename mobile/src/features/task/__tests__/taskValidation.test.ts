import {
  changeKind,
  emptyTaskForm,
  taskToForm,
  toCreateTaskRequest,
  editMinDueOn,
  toUpdateTaskRequest,
  validateTaskForm,
  type TaskFormInput,
} from '../taskValidation';

const today = '2026-11-22';
const form = (extra: Partial<TaskFormInput> = {}): TaskFormInput => ({
  ...emptyTaskForm(today),
  kind: 'watering',
  interval: { value: '7', unit: 'day' },
  ...extra,
});

describe('validateTaskForm', () => {
  it('種類と周期があれば通る。前後の空白を除く', () => {
    const result = validateTaskForm(form({ conditionText: ' 土が乾いていたら ' }), today);
    expect(result).toEqual({ ok: true, value: expect.objectContaining({ conditionText: '土が乾いていたら' }) });
  });

  it('種類が未選択なら required', () => {
    expect(validateTaskForm(form({ kind: '' }), today)).toEqual({ ok: false, errors: { kind: 'required' } });
  });

  it('「その他」は作業名が必須で、20文字まで', () => {
    expect(validateTaskForm(form({ kind: 'other', customName: '  ' }), today)).toEqual({
      ok: false,
      errors: { customName: 'required' },
    });
    expect(validateTaskForm(form({ kind: 'other', customName: 'あ'.repeat(21) }), today)).toEqual({
      ok: false,
      errors: { customName: 'too_long' },
    });
  });

  it('周期は1以上で、日は365・週は52・か月は24まで。全角の数字も読む', () => {
    expect(validateTaskForm(form({ interval: { value: '', unit: 'day' } }), today)).toEqual({
      ok: false,
      errors: { interval: 'required' },
    });
    expect(validateTaskForm(form({ interval: { value: '0', unit: 'day' } }), today)).toEqual({
      ok: false,
      errors: { interval: 'invalid' },
    });
    expect(validateTaskForm(form({ interval: { value: '53', unit: 'week' } }), today)).toEqual({
      ok: false,
      errors: { interval: 'invalid' },
    });
    expect(validateTaskForm(form({ interval: { value: '1.5', unit: 'day' } }), today)).toEqual({
      ok: false,
      errors: { interval: 'invalid' },
    });
    expect(validateTaskForm(form({ interval: { value: '２４', unit: 'month' } }), today).ok).toBe(true);
  });

  it('季節ごとの周期は、開いているときだけ確かめる', () => {
    const seasonal = {
      enabled: false,
      spring: { value: '', unit: 'day' as const },
      summer: { value: '4', unit: 'day' as const },
      autumn: { value: '7', unit: 'day' as const },
      winter: { value: '14', unit: 'day' as const },
    };
    expect(validateTaskForm(form({ seasonal }), today).ok).toBe(true);
    expect(validateTaskForm(form({ seasonal: { ...seasonal, enabled: true } }), today)).toEqual({
      ok: false,
      errors: { 'seasonal.spring': 'required' },
    });
  });

  it('季節ごとに変えるときは、基本の周期を隠すので確かめない（2026-11-22）', () => {
    const seasonal = {
      enabled: true,
      spring: { value: '7', unit: 'day' as const },
      summer: { value: '4', unit: 'day' as const },
      autumn: { value: '7', unit: 'day' as const },
      winter: { value: '2', unit: 'week' as const },
    };
    expect(validateTaskForm(form({ interval: { value: '', unit: 'day' }, seasonal }), today).ok).toBe(true);
  });

  it('予定日は必須。作成では今日より前を選べない', () => {
    expect(validateTaskForm(form({ nextDueOn: '' }), today)).toEqual({ ok: false, errors: { nextDueOn: 'required' } });
    expect(validateTaskForm(form({ nextDueOn: '2026-11-21' }), today)).toEqual({
      ok: false,
      errors: { nextDueOn: 'past' },
    });
  });

  it('実行条件は100文字、メモは200文字まで', () => {
    expect(validateTaskForm(form({ conditionText: 'あ'.repeat(101), note: 'あ'.repeat(201) }), today)).toEqual({
      ok: false,
      errors: { conditionText: 'too_long', note: 'too_long' },
    });
  });
});

// 検証を通した値（送る値を作る関数には、検証を通したものだけを渡す）
const valid = (input: TaskFormInput) => {
  const result = validateTaskForm(input, today);
  if (!result.ok) throw new Error(JSON.stringify(result.errors));
  return result.value;
};

describe('toCreateTaskRequest', () => {
  it('数にして送る。空の任意の項目と、閉じた季節ごとの周期は送らない。作業名は「その他」だけ', () => {
    expect(toCreateTaskRequest(valid(form({ customName: '葉水' })))).toEqual({
      kind: 'watering',
      interval: { value: 7, unit: 'day' },
      nextDueOn: today,
    });
  });

  it('季節ごとの周期を開いていれば、4つとも送る。必須の基本の周期には、春の周期を入れる', () => {
    const request = toCreateTaskRequest(
      valid(
        form({
          kind: 'other',
          interval: { value: '', unit: 'day' },
          customName: '葉水',
          conditionText: '乾燥していたら',
          note: '霧吹きで',
          seasonal: {
            enabled: true,
            spring: { value: '7', unit: 'day' },
            summer: { value: '4', unit: 'day' },
            autumn: { value: '7', unit: 'day' },
            winter: { value: '2', unit: 'week' },
          },
        }),
      ),
    );
    expect(request).toEqual({
      kind: 'other',
      customName: '葉水',
      interval: { value: 7, unit: 'day' },
      seasonalIntervals: {
        spring: { value: 7, unit: 'day' },
        summer: { value: 4, unit: 'day' },
        autumn: { value: 7, unit: 'day' },
        winter: { value: 2, unit: 'week' },
      },
      conditionText: '乾燥していたら',
      note: '霧吹きで',
      nextDueOn: today,
    });
  });
});

describe('toUpdateTaskRequest', () => {
  it('種類は送らない。空の任意の項目は空文字で消し、閉じた季節ごとの周期は {} で消す', () => {
    expect(toUpdateTaskRequest(form(), form())).toEqual({
      interval: { value: 7, unit: 'day' },
      seasonalIntervals: {},
      conditionText: '',
      note: '',
    });
  });

  it('予定日は変えたときだけ送る（送ると、季節の切り替えで計算し直さなくなるため）', () => {
    const initial = form({ nextDueOn: '2026-11-25' });
    expect(toUpdateTaskRequest(form({ nextDueOn: '2026-11-25' }), initial).nextDueOn).toBeUndefined();
    expect(toUpdateTaskRequest(form({ nextDueOn: '2026-11-28' }), initial).nextDueOn).toBe('2026-11-28');
  });

  it('季節ごとの周期を開いていれば、基本の周期には春の周期を入れる', () => {
    const seasonal = {
      enabled: true,
      spring: { value: '10', unit: 'day' as const },
      summer: { value: '4', unit: 'day' as const },
      autumn: { value: '7', unit: 'day' as const },
      winter: { value: '2', unit: 'week' as const },
    };
    expect(toUpdateTaskRequest(form({ seasonal }), form()).interval).toEqual({ value: 10, unit: 'day' });
  });
});

describe('taskToForm', () => {
  it('API のタスクを入力欄の値にする。季節ごとの周期がなければ、閉じて基本の周期で埋める', () => {
    const value = taskToForm({
      kind: 'watering',
      interval: { value: 7, unit: 'day' },
      seasonalIntervals: {},
      nextDueOn: '2026-11-30',
    });
    expect(value).toEqual({
      kind: 'watering',
      customName: '',
      interval: { value: '7', unit: 'day' },
      seasonal: {
        enabled: false,
        spring: { value: '7', unit: 'day' },
        summer: { value: '7', unit: 'day' },
        autumn: { value: '7', unit: 'day' },
        winter: { value: '7', unit: 'day' },
      },
      nextDueOn: '2026-11-30',
      conditionText: '',
      note: '',
    });
  });

  it('季節ごとの周期の一部だけがあれば、開いて、ない季節を基本の周期で埋める', () => {
    const value = taskToForm({
      kind: 'watering',
      interval: { value: 7, unit: 'day' },
      seasonalIntervals: { summer: { value: 4, unit: 'day' } },
      nextDueOn: '2026-11-30',
    });
    expect(value.seasonal).toEqual({
      enabled: true,
      spring: { value: '7', unit: 'day' },
      summer: { value: '4', unit: 'day' },
      autumn: { value: '7', unit: 'day' },
      winter: { value: '7', unit: 'day' },
    });
  });
});

describe('changeKind', () => {
  const defaults = [{ kind: 'watering' as const, conditionText: '土の表面が乾いたら' }];

  it('実行条件が空か、前の種類のおすすめのままなら、新しい種類のおすすめに入れ替える', () => {
    const watering = changeKind(form({ kind: '', interval: { value: '', unit: 'day' } }), 'watering', defaults);
    expect(watering.conditionText).toBe('土の表面が乾いたら');
    expect(changeKind(watering, 'pruning', defaults).conditionText).toBe('');
  });

  it('自分で書いた実行条件は残す', () => {
    const edited = form({ conditionText: '葉が垂れていたら' });
    expect(changeKind(edited, 'fertilizing', defaults).conditionText).toBe('葉が垂れていたら');
  });

  it('植え替えは、周期が空か前の種類の目安のままなら、24か月を入れる（docs/decisions/0028）', () => {
    const repotting = changeKind(form({ kind: '', interval: { value: '', unit: 'day' } }), 'repotting', defaults);
    expect(repotting.interval).toEqual({ value: '24', unit: 'month' });
    expect(changeKind(repotting, 'watering', defaults).interval).toEqual({ value: '', unit: 'day' });
    expect(changeKind(form(), 'repotting', defaults).interval).toEqual({ value: '7', unit: 'day' });
  });
});

describe('editMinDueOn', () => {
  it('今日と今の予定日の早いほう（期限切れのタスクも、予定日を変えずに保存できるように）', () => {
    expect(editMinDueOn(today, '2026-11-30')).toBe(today);
    expect(editMinDueOn(today, '2026-11-20')).toBe('2026-11-20');
    expect(editMinDueOn(today, '')).toBe(today);
  });
});
