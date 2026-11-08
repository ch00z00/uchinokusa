import { createInstance } from 'i18next';
import { initReactI18next } from 'react-i18next';

import ja from './ja.json';

// まずは日本語だけ。言語を足すときは、en.json などを足して resources に入れ、
// expo-localization の getLocales() で端末の言語から選ぶ
export const resources = {
  ja: { translation: ja },
} as const;

export const defaultLanguage = 'ja';

const i18n = createInstance();

// initReactI18next で、useTranslation() がこのインスタンスを使うようになる
void i18n.use(initReactI18next).init({
  resources,
  lng: defaultLanguage,
  fallbackLng: defaultLanguage,
  // React が文字列をエスケープするので、i18next ではしない
  interpolation: { escapeValue: false },
});

export default i18n;
