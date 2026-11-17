import { Alert } from 'react-native';

export type ConfirmOptions = {
  title: string;
  message?: string;
  /** 実行のボタンの文言（「削除」「退出」など） */
  confirmLabel: string;
  cancelLabel: string;
  /** 取り消せない操作なら true（iOS では赤いボタンになる） */
  destructive?: boolean;
};

/**
 * OS の確認のアラート（OS の層。.kiro/steering/design.md）。実行が選ばれたら true。
 * ブラウザでは React Native Web の Alert が何もしないので、confirm.web.ts でブラウザの確認を使う
 */
export function confirm({ title, message, confirmLabel, cancelLabel, destructive }: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    Alert.alert(
      title,
      message,
      [
        { text: cancelLabel, style: 'cancel', onPress: () => resolve(false) },
        { text: confirmLabel, style: destructive ? 'destructive' : 'default', onPress: () => resolve(true) },
      ],
      // Android で、アラートの外を押して閉じたとき
      { cancelable: true, onDismiss: () => resolve(false) },
    );
  });
}

/** 知らせるだけのアラート（ボタンは「OK」だけ） */
export function notify(title: string, message?: string): void {
  Alert.alert(title, message);
}
