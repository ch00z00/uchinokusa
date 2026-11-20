import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { DateField } from '@/components/DateField';
import { SuggestionCard } from '@/components/SuggestionCard';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { findInitialRepotting, repottingReason, type Suggestion } from '@/features/suggestion/suggestionText';
import { useAcceptSuggestion, useDismissSuggestion, useSuggestions } from '@/features/suggestion/useSuggestions';
import { formatDayWithYear } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// PLANT-04 植え替えの提案（要件17-1、17-4〜17-7）。購入日つきで植物を登録したあとに、登録のモーダルを置き換えて出す
export default function RepottingSuggestionScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const suggestions = useSuggestions(householdId);
  const suggestion = suggestions.data ? findInitialRepotting(suggestions.data, id) : undefined;

  // 提案がなければ（植え替えのタスクがすでにあるなど）、何も出さずに閉じる
  const none = suggestions.data !== undefined && suggestion === undefined;
  useEffect(() => {
    if (none) router.back();
  }, [none]);

  if (suggestion) return <Repotting householdId={householdId} suggestion={suggestion} />;
  return (
    <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
      {suggestions.isError ? (
        <>
          <AppText color="secondary">{t(errorMessageKey(suggestions.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void suggestions.refetch()} />
        </>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </View>
  );
}

function Repotting({ householdId, suggestion }: { householdId: string; suggestion: Suggestion }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const accept = useAcceptSuggestion(householdId);
  const dismiss = useDismissSuggestion(householdId);
  const proposed = suggestion.proposedDate ?? '';
  // 「日付を変えて入れる」を選んだら、日付の欄を出す（要件17-5）
  const [changing, setChanging] = useState(false);
  const [date, setDate] = useState(proposed);
  const busy = accept.isPending || dismiss.isPending;

  const plantName = typeof suggestion.messageParams.plantName === 'string' ? suggestion.messageParams.plantName : '';
  const done = () => router.back();
  const accepted = () => {
    toast.show(t('suggestion.repotting.accepted'));
    done();
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.lg }}
    >
      <AppText variant="title3">{t('suggestion.repotting.title')}</AppText>
      <SuggestionCard
        kind={suggestion.kind}
        message={t('suggestion.repotting.message', { plantName, date: formatDayWithYear(proposed) })}
        reason={repottingReason(suggestion.reasonParams, t)}
      >
        {changing ? (
          <>
            <DateField label={t('suggestion.repotting.date')} value={date} onChange={setDate} clearable={false} />
            <Button
              title={t('suggestion.repotting.acceptWithDate')}
              loading={accept.isPending}
              disabled={busy}
              onPress={() => accept.mutate({ suggestionId: suggestion.id, date }, { onSuccess: accepted })}
            />
          </>
        ) : (
          <>
            <Button
              title={t('suggestion.repotting.accept')}
              loading={accept.isPending}
              disabled={busy}
              onPress={() => accept.mutate({ suggestionId: suggestion.id }, { onSuccess: accepted })}
            />
            <Button
              title={t('suggestion.repotting.changeDate')}
              variant="secondary"
              disabled={busy}
              onPress={() => setChanging(true)}
            />
          </>
        )}
        <Button
          title={t('suggestion.repotting.dismiss')}
          variant="secondary"
          loading={dismiss.isPending}
          disabled={busy}
          onPress={() => dismiss.mutate(suggestion.id, { onSuccess: done })}
        />
      </SuggestionCard>
      <AppText variant="footnote" color="secondary">
        {t('suggestion.repotting.dismissNote')}
      </AppText>
      {(accept.isError || dismiss.isError) && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(accept.error ?? dismiss.error))}
        </AppText>
      )}
    </ScrollView>
  );
}
