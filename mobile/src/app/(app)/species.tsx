import { router, Stack } from 'expo-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, FlatList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { currentSpeciesRequest, pickSpecies } from '@/features/plant/speciesPicker';
import type { SpeciesValue } from '@/features/plant/plantValidation';
import { useSpeciesSearch } from '@/features/plant/useSpecies';
import { useDebouncedValue } from '@/lib/useDebouncedValue';
import { useTheme } from '@/theme';

// PLANT-03 種類の検索（要件6-2、6-3）。植物の登録・編集の種類の欄から開く OS のシート
export default function SpeciesScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const [request] = useState(currentSpeciesRequest);
  const [query, setQuery] = useState('');
  const q = useDebouncedValue(query.trim(), 300);
  const species = useSpeciesSearch(q);

  // アプリを開き直したあとなどで依頼がなければ、何も選ばずに閉じる
  useEffect(() => {
    if (!request) router.back();
  }, [request]);
  if (!request) return null;

  const choose = (value: SpeciesValue) => {
    pickSpecies(value);
    router.back();
  };
  const typed = query.trim();
  const current = request.value;

  return (
    <>
      <Stack.Screen
        options={{
          headerSearchBarOptions: {
            placeholder: t('species.searchPlaceholder'),
            autoFocus: true,
            hideWhenScrolling: false,
            onChangeText: (e) => setQuery(e.nativeEvent.text),
            onCancelButtonPress: () => setQuery(''),
          },
        }}
      />
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
        data={species.data ?? []}
        keyExtractor={(item) => item.id}
        ListHeaderComponent={
          <View>
            {/* 見つからない種類は、入れた言葉をそのまま種類にできる（要件6-3） */}
            {typed !== '' && (
              <ListRow
                title={t('species.useFreeText', { text: typed })}
                accent
                onPress={() => choose({ kind: 'free', text: typed })}
              />
            )}
            {current.kind !== 'none' && typed === '' && (
              <ListRow title={t('species.clear')} accent onPress={() => choose({ kind: 'none' })} />
            )}
          </View>
        }
        renderItem={({ item }) => (
          <ListRow
            title={item.nameJa}
            subtitle={[item.scientificName, ...item.aliases].filter(Boolean).join(t('species.separator'))}
            selected={current.kind === 'species' && current.id === item.id}
            onPress={() => choose({ kind: 'species', id: item.id, name: item.nameJa })}
          />
        )}
        ListEmptyComponent={
          <View style={{ padding: theme.space.lg }}>
            {species.isError ? (
              <AppText color="secondary">{t(errorMessageKey(species.error))}</AppText>
            ) : species.data ? (
              <AppText color="secondary">{t('species.noMatch')}</AppText>
            ) : (
              <ActivityIndicator color={theme.color.text.secondary} />
            )}
          </View>
        }
      />
    </>
  );
}
