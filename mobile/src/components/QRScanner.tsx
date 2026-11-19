import { CameraView, useCameraPermissions } from 'expo-camera';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Linking, StyleSheet, View } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';
import { Button } from './Button';

type Props = {
  /** 読み取った QR コードの中身。同じ QR を映している間は、何度も呼ばれる */
  onScan: (data: string) => void;
  /** 映像の下に出す案内（「招待の QR コードではありません」など） */
  message: string;
};

/**
 * QR コードの読み取り（foundations.md の 5章の QRScanner）。
 * カメラの許可は OS のダイアログで求め、拒否されたあとは端末の設定を開く導線を出す
 */
export function QRScanner({ onScan, message }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [permission, requestPermission] = useCameraPermissions();

  if (!permission) {
    return (
      <View style={[styles.center, { backgroundColor: theme.color.bg.base }]}>
        <ActivityIndicator color={theme.color.text.secondary} />
      </View>
    );
  }

  if (!permission.granted) {
    // まだ聞いていない（または聞き直せる）ときは OS のダイアログを出し、拒否されたあとは設定アプリへ
    const canAsk = permission.canAskAgain;
    return (
      <View
        style={[styles.center, { backgroundColor: theme.color.bg.base, padding: theme.space.lg, gap: theme.space.xl }]}
      >
        <AppText color="secondary" style={styles.text}>
          {t(canAsk ? 'scan.permission.lead' : 'scan.permission.denied')}
        </AppText>
        <Button
          title={t(canAsk ? 'scan.permission.allow' : 'scan.permission.openSettings')}
          onPress={() => void (canAsk ? requestPermission() : Linking.openSettings())}
        />
      </View>
    );
  }

  return (
    <View style={styles.fill}>
      <CameraView
        style={styles.fill}
        facing="back"
        barcodeScannerSettings={{ barcodeTypes: ['qr'] }}
        onBarcodeScanned={({ data }) => onScan(data)}
      />
      <View
        style={{
          position: 'absolute',
          left: theme.space.lg,
          right: theme.space.lg,
          bottom: theme.space['2xl'],
          padding: theme.space.md,
          borderRadius: theme.radius.md,
          backgroundColor: theme.color.bg.surface,
        }}
      >
        <AppText style={styles.text}>{message}</AppText>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  text: { textAlign: 'center' },
});
