import { defaultLanguage, supportedLanguages } from './i18nList';

const traditionalChineseMarkers = ['zh_hk', 'zh_tw', 'zh_mo', 'zh_hant'];
const simplifiedChineseMarkers = ['zh_cn', 'zh_sg', 'zh_my', 'zh_hans'];

const hasMarker = (value, markers) => markers.some((marker) => value === marker || value.startsWith(`${marker}_`));

export { defaultLanguage, supportedLanguages };

export const normalizeLanguageCode = (language) => {
  if (!language || typeof language !== 'string') {
    return '';
  }

  const normalized = language.trim().replace(/-/g, '_');
  if (!normalized) {
    return '';
  }

  if (supportedLanguages.includes(normalized)) {
    return normalized;
  }

  const lower = normalized.toLowerCase();

  if (lower === 'en' || lower.startsWith('en_')) {
    return 'en_US';
  }

  if (lower === 'ja' || lower.startsWith('ja_')) {
    return 'ja_JP';
  }

  if (hasMarker(lower, traditionalChineseMarkers)) {
    return 'zh_HK';
  }

  if (lower === 'zh' || hasMarker(lower, simplifiedChineseMarkers) || lower.startsWith('zh_')) {
    return 'zh_CN';
  }

  return normalized;
};

export const matchSupportedLanguage = (language) => {
  const normalized = normalizeLanguageCode(language);

  return supportedLanguages.includes(normalized) ? normalized : '';
};

export const resolvePreferredLanguage = (...languages) => {
  for (const language of languages) {
    const matched = matchSupportedLanguage(language);
    if (matched) {
      return matched;
    }
  }

  return defaultLanguage;
};
