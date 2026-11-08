import 'i18next';

import type { resources } from './index';

// t('...') のキーを ja.json から型で確かめる（存在しないキーは型エラーになる）
declare module 'i18next' {
  interface CustomTypeOptions {
    resources: (typeof resources)['ja'];
  }
}
