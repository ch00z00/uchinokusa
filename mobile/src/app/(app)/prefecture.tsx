import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { FlatList } from 'react-native';

import { ListRow } from '@/components/ListRow';
import { pickPrefecture } from '@/features/household/prefecturePicker';
import { prefectures } from '@/features/household/prefectures';

// 都道府県の選択（OS のシート）。世帯の作成（ONB-02）と世帯の設定（HH-01）の地域の欄から開く。
// 一覧が長いので、メニュー（アクションシート）ではなくシートにする。下に引くと、選ばずに閉じる
export default function PrefectureScreen() {
  const { t } = useTranslation();
  const { value } = useLocalSearchParams<{ value?: string }>();

  return (
    <FlatList
      contentInsetAdjustmentBehavior="automatic"
      data={['', ...prefectures]}
      keyExtractor={(item) => item || 'unset'}
      renderItem={({ item }) => (
        <ListRow
          title={item || t('household.form.unset')}
          selected={item === (value ?? '')}
          onPress={() => {
            pickPrefecture(item);
            router.back();
          }}
        />
      )}
    />
  );
}
