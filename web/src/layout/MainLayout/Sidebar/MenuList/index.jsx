// material-ui
import { Typography } from '@mui/material';

// project imports
import NavGroup from './NavGroup';
import menuItem from 'menu-items';
import { useIsAdmin } from 'utils/common';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

// ==============================|| SIDEBAR MENU LIST ||============================== //
const MenuList = ({ isMini = false }) => {
  const userIsAdmin = useIsAdmin();
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  const translateItem = (item) => {
    const newItem = { ...item };
    if (newItem.title) {
      newItem.title = t(newItem.title);
    } else if (newItem.id) {
      newItem.title = t(newItem.id);
    }
    if (newItem.children) {
      newItem.children = newItem.children.map(translateItem);
    }
    return newItem;
  };

  const navItems = menuItem.items.map((item) => {
    const translatedItem = translateItem(item);

    if (translatedItem.type !== 'group') {
      return (
        <Typography key={translatedItem.id} variant="h6" color="error" align="center">
          {t('menu.error')}
        </Typography>
      );
    }

    const filteredChildren = translatedItem.children.filter(
      (child) => (!child.isAdmin || userIsAdmin) && !(siteInfo.UserInvoiceMonth === false && child.id === 'invoice')
    );

    if (filteredChildren.length === 0) {
      return null;
    }

    return <NavGroup key={translatedItem.id} item={{ ...translatedItem, children: filteredChildren }} isMini={isMini} />;
  });

  return <>{navItems}</>;
};

export default MenuList;
