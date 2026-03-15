import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { resources } from './resources';
import LanguageDetector from 'i18next-browser-languagedetector';
import { defaultLanguage, normalizeLanguageCode, supportedLanguages } from './language';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: defaultLanguage,
    supportedLngs: supportedLanguages,
    debug: false,
    detection: {
      order: ['localStorage', 'navigator', 'htmlTag'],
      lookupLocalStorage: 'appLanguage',
      caches: [],
      convertDetectedLanguage: normalizeLanguageCode
    },
    interpolation: {
      escapeValue: false
    }
  });

export default i18n;
