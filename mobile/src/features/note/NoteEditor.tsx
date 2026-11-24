import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { ApiError } from '@/api/errors';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { FormField } from '@/components/FormField';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import { useTheme } from '@/theme';

import { maxNoteLength, noteLength, validateNote } from './note';

type Props = {
  initialBody: string;
  pending: boolean;
  error: unknown;
  /** 保存（前後の空白を除いた本文を渡す）。成功したら true */
  onSave: (body: string) => Promise<boolean>;
  /** 編集のときだけ渡す。削除の行を出す */
  onDelete?: () => void;
  deleting?: boolean;
  deleteError?: unknown;
};

/**
 * メモの作成・編集のフォーム（PLANT-14、要件13-1・13-3・13-4）。
 * 複数行の入力欄と文字数、保存のボタン。編集のときは下に削除の行
 */
export function NoteEditor({ initialBody, pending, error, onSave, onDelete, deleting, deleteError }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const [body, setBody] = useState(initialBody);
  // 入力のはじめから「入力してください」を出さないように、保存を押してから確かめる。文字数の超過は、文字数の色ですぐ示す
  const [submitted, setSubmitted] = useState(false);
  const length = noteLength(body);
  const localError = validateNote(body);
  const serverError = serverBodyError(error);
  const fieldError = submitted && localError ? localError : serverError;

  const save = async () => {
    setSubmitted(true);
    if (localError) return;
    if (await onSave(body.trim())) {
      toast.show(t('common.saved'));
      router.back();
    }
  };

  const otherError = error && !serverError ? t(errorMessageKey(error)) : undefined;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space['2xl'] }}
    >
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.lg }}>
        <FormField
          label={t('notes.body')}
          placeholder={t('notes.placeholder')}
          value={body}
          onChangeText={setBody}
          rows={5}
          count={{ length, max: maxNoteLength }}
          error={fieldError ? t(`notes.errors.${fieldError}`) : undefined}
          autoFocus
        />
        {otherError && (
          <AppText variant="footnote" color="danger">
            {otherError}
          </AppText>
        )}
        <Button title={t('common.save')} onPress={() => void save()} loading={pending} disabled={deleting} />
      </View>
      {onDelete && (
        <View>
          <ListRow title={t('notes.delete.action')} destructive disabled={pending || deleting} onPress={onDelete} />
          {deleteError !== null && deleteError !== undefined && (
            <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
              {t(errorMessageKey(deleteError))}
            </AppText>
          )}
        </View>
      )}
    </ScrollView>
  );
}

/** サーバーが返した本文の誤り（fields.body）。欄の下に出す */
function serverBodyError(error: unknown): 'required' | 'too_long' | undefined {
  if (!(error instanceof ApiError)) return undefined;
  const code = error.fields.body;
  return code === 'required' || code === 'too_long' ? code : undefined;
}
