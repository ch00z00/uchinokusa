import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FlatList, Modal, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useTheme } from '@/theme';

import { prefectures } from './prefectures';

type Props = {
  /** 選んだ都道府県。空文字は未設定 */
  value: string;
  onChange: (value: string) => void;
};

/**
 * 都道府県を一覧から選ぶ欄。押すと、OS のシート（iOS の pageSheet）で47の一覧と「未設定」を出す。
 * 一覧が長いので、メニュー（アクションシート）ではなくシートにする
 */
export function PrefectureField({ value, onChange }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const choose = (next: string) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {t('household.form.prefecture')}
      </AppText>
      <View style={{ borderRadius: theme.radius.md, overflow: 'hidden' }}>
        <ListRow
          title={value || t('household.form.unset')}
          navigates
          onPress={() => setOpen(true)}
          accessibilityHint={t('household.form.prefectureHint')}
        />
      </View>
      <Modal visible={open} animationType="slide" presentationStyle="pageSheet" onRequestClose={() => setOpen(false)}>
        <SafeAreaView edges={['bottom']} style={{ flex: 1, backgroundColor: theme.color.bg.base }}>
          <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
            <AppText variant="headline">{t('household.form.prefecture')}</AppText>
            <Button title={t('common.cancel')} variant="secondary" onPress={() => setOpen(false)} />
          </View>
          <FlatList
            data={['', ...prefectures]}
            keyExtractor={(item) => item || 'unset'}
            renderItem={({ item }) => (
              <ListRow
                title={item || t('household.form.unset')}
                selected={item === value}
                onPress={() => choose(item)}
              />
            )}
          />
        </SafeAreaView>
      </Modal>
    </View>
  );
}
