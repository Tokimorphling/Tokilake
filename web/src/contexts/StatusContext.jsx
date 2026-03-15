import { useEffect, useCallback, createContext } from 'react';
import { API } from 'utils/api';
import { showNotice, showError } from 'utils/common';
import { SET_SITE_INFO, SET_MODEL_OWNEDBY } from 'store/actions';
import { useDispatch } from 'react-redux';
import { useTranslation } from 'react-i18next';
import i18n from 'i18next';

export const LoadStatusContext = createContext();

// eslint-disable-next-line
const StatusProvider = ({ children }) => {
  const { t } = useTranslation();
  const dispatch = useDispatch();

  const loadStatus = useCallback(async () => {
    let system_name = '';
    let analytics_code = '';
    try {
      const res = await API.get('/api/status');
      const { success, data } = res.data;
      if (success) {
        if (!data.chat_link) {
          delete data.chat_link;
        }
        // 设置系统默认语言
        // 优先级：用户选择 (localStorage) > 浏览器检测语言 > 服务器设置 > 默认 zh_CN
        const userLanguage = localStorage.getItem('appLanguage');
        const serverLanguage = data.language;
        const detectedLanguage = i18n.language || 'zh_CN';
        // alert(userLanguage);
        console.log('userLanguage: %s, detecteted: %s', userLanguage, detectedLanguage);
        // 如果用户没有手动选择语言，则使用检测到的浏览器语言或服务器设置
        let storedLanguage;
        if (userLanguage) {
          storedLanguage = userLanguage;
        } else if (detectedLanguage) {
          // 使用检测到的浏览器语言，如果不在支持列表中则回退到 zh_CN
          storedLanguage = detectedLanguage;
        } else if (serverLanguage) {
          storedLanguage = serverLanguage;
        } else {
          storedLanguage = 'zh_CN';
        }
        
        localStorage.setItem('default_language', storedLanguage);
        i18n.changeLanguage(storedLanguage);
        localStorage.setItem('siteInfo', JSON.stringify(data));
        localStorage.setItem('quota_per_unit', data.quota_per_unit);
        localStorage.setItem('display_in_currency', data.display_in_currency);
        dispatch({ type: SET_SITE_INFO, payload: data });
        if (
          data.version !== import.meta.env.VITE_APP_VERSION &&
          data.version !== 'v0.0.0' &&
          data.version !== '' &&
          import.meta.env.VITE_APP_VERSION !== ''
        ) {
          showNotice(t('common.unableServerTip', { version: data.version }));
        }
        if (data.system_name) {
          system_name = data.system_name;
        }
        if (data.analytics_code) {
          analytics_code = data.analytics_code;
        }
      } else {
        const backupSiteInfo = localStorage.getItem('siteInfo');
        if (backupSiteInfo) {
          const data = JSON.parse(backupSiteInfo);
          if (data.system_name) {
            system_name = data.system_name;
          }
          dispatch({
            type: SET_SITE_INFO,
            payload: data
          });
        }
      }
    } catch (error) {
      showError(t('common.unableServer'));
    }

    if (system_name) {
      document.title = system_name;
    }

    if (analytics_code) {
      // Check if the script is already injected
      if (!document.getElementById('analytics-code')) {
        const range = document.createRange();
        const fragment = range.createContextualFragment(analytics_code);
        // Add an ID to the first child to prevent duplicate injection
        if (fragment.firstElementChild) {
          fragment.firstElementChild.id = 'analytics-code';
          document.head.appendChild(fragment);
        }
      }
    }
    // eslint-disable-next-line
  }, [dispatch]);

  const loadOwnedby = useCallback(async () => {
    try {
      const res = await API.get('/api/model_ownedby');
      const { success, data } = res.data;
      if (success) {
        dispatch({ type: SET_MODEL_OWNEDBY, payload: data });
      }
    } catch (error) {
      showError(error.message);
    }
  }, [dispatch]);

  useEffect(() => {
    loadStatus().then();
    loadOwnedby();
  }, [loadStatus, loadOwnedby]);

  return <LoadStatusContext.Provider value={loadStatus}> {children} </LoadStatusContext.Provider>;
};

export default StatusProvider;
