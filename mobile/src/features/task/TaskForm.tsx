import { SegmentedControl } from '@expo/ui/community/segmented-control';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Keyboard, StyleSheet, View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { CareKindIcon } from '@/components/CareKindIcon';
import { DateField } from '@/components/DateField';
import { FormField } from '@/components/FormField';
import { IntervalInput } from '@/components/IntervalInput';
import { SeasonalIntervalEditor } from '@/components/SeasonalIntervalEditor';
import { useTheme } from '@/theme';

import { careKinds, seasons, type CareKind, type CareTaskDefault, type Season } from './careTask';
import {
  changeKind,
  isRecommendedCondition,
  validateTaskForm,
  type TaskField,
  type TaskFieldError,
  type TaskFormInput,
} from './taskValidation';

type Props = {
  initial: TaskFormInput;
  /** 植物マスタのおすすめ（実行条件の初期値。要件6-4） */
  defaults: readonly CareTaskDefault[];
  /** 作成なら true（作業の種類を選べる。作成したあとは変えられない） */
  creating: boolean;
  /** 予定日の欄の見出し（作成は「初回の予定日」） */
  dueLabel: string;
  /** 選べる最初の予定日（`YYYY-MM-DD`）。作成では今日 */
  minDueOn?: string;
  submitLabel: string;
  onSubmit: (value: TaskFormInput & { kind: CareKind }) => void;
  pending: boolean;
  /** 送信の失敗（422 の入力のエラーは、入力欄の下に出す） */
  error: unknown;
};

/** サーバーの入力のエラーの項目名（`seasonalIntervals.summer` など）を、入力欄にあてはめる */
function serverField(field: string): TaskField | undefined {
  const seasonal = /^seasonalIntervals\.(\w+)$/.exec(field)?.[1];
  if (seasonal) return seasons.includes(seasonal as Season) ? `seasonal.${seasonal as Season}` : undefined;
  const known: TaskField[] = ['kind', 'customName', 'interval', 'nextDueOn', 'conditionText', 'note'];
  return known.includes(field as TaskField) ? (field as TaskField) : undefined;
}

/** お手入れタスクの入力（TASK-01 の作成と編集で使う。要件8-1〜8-6） */
export function TaskForm(props: Props) {
  const { initial, defaults, creating, dueLabel, minDueOn, submitLabel, onSubmit, pending, error } = props;
  const { t } = useTranslation();
  const theme = useTheme();
  const [input, setInput] = useState(initial);
  const [inputErrors, setInputErrors] = useState<Partial<Record<TaskField, TaskFieldError>>>({});

  const update = (next: TaskFormInput, fields: TaskField[]) => {
    setInput(next);
    // 入力を直したら、その欄のエラーを消す
    setInputErrors((errors) =>
      Object.fromEntries(Object.entries(errors).filter(([f]) => !fields.includes(f as TaskField))),
    );
  };

  const submit = () => {
    const result = validateTaskForm(input, minDueOn);
    if (!result.ok) {
      setInputErrors(result.errors);
      return;
    }
    setInputErrors({});
    // 送ったら入力欄のフォーカスを外す（キーボードを閉じ、結果を見えるようにする）
    Keyboard.dismiss();
    onSubmit(result.value);
  };

  const serverErrors: Partial<Record<TaskField, string>> = {};
  if (error instanceof ApiError && error.code === 'validation_failed') {
    for (const [field, code] of Object.entries(error.fields)) {
      const key = serverField(field);
      if (key) serverErrors[key] = code;
    }
  }
  const fieldError = (field: TaskField): string | null => {
    const code = inputErrors[field] ?? serverErrors[field];
    if (code === undefined) return null;
    const base = field.startsWith('seasonal.') ? 'interval' : field;
    const key = `tasks.form.errors.${base}.${code}`;
    return t(key, { defaultValue: t('errors.validation_failed') });
  };
  const hasFieldError = Object.keys(inputErrors).length > 0 || Object.keys(serverErrors).length > 0;
  const kind = input.kind;

  return (
    <View style={{ gap: theme.space.lg }}>
      <View style={{ gap: theme.space.xs }}>
        <AppText variant="subheadline" color="secondary">
          {t('tasks.form.kind')}
        </AppText>
        {creating ? (
          // 作業の種類は OS のセグメントの切り替えで選ぶ。選んだ種類はアイコンと色でも示す
          <SegmentedControl
            accessibilityLabel={t('tasks.form.kind')}
            values={careKinds.map((k) => t(`care.kind.${k}`))}
            selectedIndex={kind === '' ? -1 : careKinds.indexOf(kind)}
            onChange={(e) => {
              const next = careKinds[e.nativeEvent.selectedSegmentIndex];
              if (next) update(changeKind(input, next, defaults), ['kind', 'interval', 'conditionText']);
            }}
          />
        ) : null}
        {kind !== '' && (
          <View style={[styles.row, { gap: theme.space.sm, paddingTop: theme.space.xs }]}>
            <CareKindIcon kind={kind} size="md" />
            <AppText variant="headline">{t(`care.kind.${kind}`)}</AppText>
          </View>
        )}
        {fieldError('kind') && (
          <AppText variant="footnote" color="danger">
            {fieldError('kind')}
          </AppText>
        )}
      </View>
      {kind === 'other' && (
        <FormField
          label={t('tasks.form.customName')}
          placeholder={t('tasks.form.customNamePlaceholder')}
          value={input.customName}
          onChangeText={(customName) => update({ ...input, customName }, ['customName'])}
          error={fieldError('customName')}
          returnKeyType="done"
        />
      )}
      <IntervalInput
        label={t('tasks.form.interval')}
        value={input.interval}
        onChange={(interval) => update({ ...input, interval }, ['interval'])}
        error={fieldError('interval')}
      />
      <SeasonalIntervalEditor
        value={input.seasonal}
        onChange={(seasonal) => {
          // 開いたとき、空の季節は基本の周期で埋める（そこから直せばよいように）
          const filled =
            seasonal.enabled && !input.seasonal.enabled
              ? {
                  ...seasonal,
                  ...Object.fromEntries(
                    seasons.map((s) => [s, seasonal[s].value === '' ? input.interval : seasonal[s]]),
                  ),
                }
              : seasonal;
          update(
            { ...input, seasonal: filled },
            seasons.map((s) => `seasonal.${s}` as const),
          );
        }}
        errorOf={(season) => fieldError(`seasonal.${season}`)}
      />
      <DateField
        label={dueLabel}
        value={input.nextDueOn}
        onChange={(nextDueOn) => update({ ...input, nextDueOn }, ['nextDueOn'])}
        minimumDate={minDueOn}
        clearable={false}
        error={fieldError('nextDueOn')}
      />
      <AppText variant="footnote" color="secondary">
        {t('tasks.form.optionalNote')}
      </AppText>
      <View style={{ gap: theme.space.xs }}>
        <FormField
          label={t('tasks.form.conditionText')}
          placeholder={t('tasks.form.conditionPlaceholder')}
          value={input.conditionText}
          onChangeText={(conditionText) => update({ ...input, conditionText }, ['conditionText'])}
          error={fieldError('conditionText')}
          returnKeyType="done"
        />
        {/* 植物マスタのおすすめのままなら、そう示す（要件6-4） */}
        {isRecommendedCondition(input, defaults) && (
          <AppText variant="footnote" color="secondary">
            {t('tasks.form.recommended')}
          </AppText>
        )}
      </View>
      <FormField
        label={t('tasks.form.note')}
        placeholder={t('tasks.form.notePlaceholder')}
        value={input.note}
        onChangeText={(note) => update({ ...input, note }, ['note'])}
        error={fieldError('note')}
        multiline
      />
      {error !== null && error !== undefined && !hasFieldError && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(error))}
        </AppText>
      )}
      <Button title={submitLabel} onPress={submit} loading={pending} />
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
