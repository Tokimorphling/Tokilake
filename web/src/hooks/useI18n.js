import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { defaultLanguage, matchSupportedLanguage } from 'i18n/language';

const useI18n = () => {
  const { i18n } = useTranslation();

  useEffect(() => {
    const handleLanguageChange = (lang) => {
      const nextLanguage = matchSupportedLanguage(lang) || matchSupportedLanguage(i18n.resolvedLanguage) || defaultLanguage;
      localStorage.setItem('appLanguage', nextLanguage);
    };

    i18n.on('languageChanged', handleLanguageChange);

    return () => {
      i18n.off('languageChanged', handleLanguageChange);
    };
  }, [i18n]);

  return i18n;
};

export default useI18n;
