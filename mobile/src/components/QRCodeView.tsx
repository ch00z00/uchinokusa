import { useWindowDimensions, View } from 'react-native';
import QRCode from 'react-native-qrcode-svg';

import { useTheme } from '@/theme';

const maxSize = 240;

/**
 * QR コードの表示（アプリの層。foundations.md の 5章の QRCodeView）。
 * ダークでも明るい地に暗い模様にする（color/qr/*）。周りの余白は、読み取りに必要な余白を兼ねる
 */
export function QRCodeView({ value, accessibilityLabel }: { value: string; accessibilityLabel: string }) {
  const theme = useTheme();
  const { width } = useWindowDimensions();
  // 画面の左右の余白と、QR の周りの余白を引いた幅に収める
  const size = Math.min(maxSize, width - theme.space.lg * 4);
  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={accessibilityLabel}
      style={{
        alignSelf: 'center',
        padding: theme.space.lg,
        borderRadius: theme.radius.lg,
        backgroundColor: theme.color.qr.background,
      }}
    >
      <QRCode value={value} size={size} color={theme.color.qr.foreground} backgroundColor={theme.color.qr.background} />
    </View>
  );
}
