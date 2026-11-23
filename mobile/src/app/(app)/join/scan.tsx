import { router, Stack } from 'expo-router';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';

import { AppText } from '@/components/AppText';
import { useHeaderHitArea } from '@/components/HeaderTextButton';
import { QRScanner } from '@/components/QRScanner';
import { inviteTokenFromScan } from '@/features/invitation/inviteLink';

// INV-11 QR コードの読み取り。招待の QR なら参加の確認（INV-12）へ、そうでなければ案内を出して読み取りを続ける
export default function ScanScreen() {
  const { t } = useTranslation();
  const hitArea = useHeaderHitArea();
  const [notInvite, setNotInvite] = useState(false);
  // 同じ QR を映している間、何度も呼ばれるので、一度だけ進む
  const done = useRef(false);

  const onScan = (data: string) => {
    if (done.current) return;
    const token = inviteTokenFromScan(data);
    if (token === null) {
      setNotInvite(true);
      return;
    }
    done.current = true;
    router.replace(`/invite/${token}`);
  };

  return (
    <>
      {/* 画面全体のモーダルは下に引いて閉じられないので、閉じるボタンを置く */}
      <Stack.Screen
        options={{
          headerLeft: () => (
            <Pressable accessibilityRole="button" style={hitArea} onPress={() => router.back()}>
              <AppText color="accent">{t('common.cancel')}</AppText>
            </Pressable>
          ),
        }}
      />
      <QRScanner onScan={onScan} message={t(notInvite ? 'scan.notInvite' : 'scan.lead')} />
    </>
  );
}
