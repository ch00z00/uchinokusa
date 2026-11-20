import { router, Stack } from 'expo-router';
import { useEffect, useState } from 'react';
import { FlatList } from 'react-native';

import { ListRow } from '@/components/ListRow';
import { currentChoiceRequest, pickChoice } from '@/lib/choicePicker';

// 選択肢のシート（OS のシート。foundations.md の 5章の ChoiceSheet）。ChoiceField から開く。
// 都道府県のように一覧が長くても選べるよう、メニューではなくシートにする。下に引くと、選ばずに閉じる
export default function ChoiceScreen() {
  // 開いたときの依頼を持つ（選んだあとに依頼が消えても、閉じる途中の表示が変わらないように）
  const [request] = useState(currentChoiceRequest);

  // アプリを開き直したあとなどで依頼がなければ、何も選ばずに閉じる
  useEffect(() => {
    if (!request) router.back();
  }, [request]);
  if (!request) return null;

  return (
    <>
      <Stack.Screen options={{ title: request.title }} />
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        data={request.options}
        keyExtractor={(item) => item.value || 'unset'}
        renderItem={({ item }) => (
          <ListRow
            title={item.label}
            selected={item.value === request.value}
            onPress={() => {
              pickChoice(item.value);
              router.back();
            }}
          />
        )}
        ListFooterComponent={
          request.action ? <ListRow title={request.action.label} accent onPress={request.action.onPress} /> : undefined
        }
      />
    </>
  );
}
