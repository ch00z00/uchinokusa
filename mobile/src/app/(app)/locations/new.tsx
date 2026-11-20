import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { emptyLocationForm } from '@/features/location/locationValidation';
import { LocationForm } from '@/features/location/LocationForm';
import { notifyLocationCreated } from '@/features/location/newLocationFlow';
import { useCreateLocation } from '@/features/location/useLocations';
import { useTheme } from '@/theme';

// LOC-02 置き場所の作成（要件7-1、7-2）。置き場所の一覧（LOC-01）と、植物の登録・編集の置き場所の欄から開くモーダル
export default function NewLocationScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <NewLocation householdId={state.household.id} />;
}

function NewLocation({ householdId }: { householdId: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const create = useCreateLocation(householdId);

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ padding: theme.space.lg }}
    >
      <LocationForm
        initial={emptyLocationForm}
        submitLabel={t('location.create.submit')}
        pending={create.isPending}
        error={create.error}
        onSubmit={(value) =>
          create.mutate(value, {
            onSuccess: (created) => {
              // 植物の登録・編集から開いたときは、作った置き場所をその植物の欄で選ぶ
              notifyLocationCreated(created.id);
              toast.show(t('common.saved'));
              router.back();
            },
          })
        }
      />
    </ScrollView>
  );
}
