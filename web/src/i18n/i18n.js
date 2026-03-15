import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { resources } from './resources';
import LanguageDetector from 'i18next-browser-languagedetector';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'zh_CN',
    debug: false,
    // 移除固定的 lng，让 LanguageDetector 自动检测浏览器语言
    // lng: 'zh_CN',
    interpolation: {
      escapeValue: false
    },
    // LanguageDetector 配置
    detection: {
      order: ['localStorage', 'navigator', 'htmlTag'],
      caches: ['localStorage']
    }
  });

export default i18n;
