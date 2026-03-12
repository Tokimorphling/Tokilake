import React, { useState } from 'react';
import { useTheme } from '@mui/material/styles';
import { Avatar, Box, ButtonBase, Menu, MenuItem, Typography } from '@mui/material';
import i18nList from 'i18n/i18nList';
import useI18n from 'hooks/useI18n';
import { Icon } from '@iconify/react';

export default function I18nButton() {
  const theme = useTheme();
  const i18n = useI18n();

  const [anchorEl, setAnchorEl] = useState(null);

  const handleMenuOpen = (event) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const handleLanguageChange = (lng) => {
    i18n.changeLanguage(lng);
    handleMenuClose();
  };

  return (
    <Box
      sx={{
        ml: 2,
        mr: 3,
        [theme.breakpoints.down('md')]: {
          mr: 2
        }
      }}
    >
      <ButtonBase sx={{ borderRadius: '12px' }} onClick={handleMenuOpen}>
        <Avatar
          variant="rounded"
          sx={{
            ...theme.typography.commonAvatar,
            ...theme.typography.mediumAvatar,
            ...theme.typography.menuButton,
            transition: 'all .2s ease-in-out',
            borderColor: theme.typography.menuChip.background,
            borderRadius: '50%',
            background: 'transparent',
            overflow: 'hidden',
            '&[aria-controls="menu-list-grow"],&:hover': {
              boxShadow: '0 4px 8px rgba(0,0,0,0.15)',
              background: 'transparent !important'
            }
          }}
          color="inherit"
        >
          <Box
            sx={{
              width: '1.8rem',
              height: '1.8rem',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <Icon icon="mdi:language" width="24" height="24" color={theme.palette.text.primary} />
          </Box>
        </Avatar>
      </ButtonBase>
      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleMenuClose}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'center'
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'center'
        }}
      >
        {i18nList.map((item) => (
          <MenuItem
            key={item.lng}
            onClick={() => handleLanguageChange(item.lng)}
            sx={{
              display: 'flex',
              alignItems: 'center',
              px: 2,
              py: 1.2,
              minWidth: '120px'
            }}
          >
            <Typography variant="body1" sx={{ fontWeight: i18n.language === item.lng ? 600 : 400 }}>
              {item.name}
            </Typography>
          </MenuItem>
        ))}
      </Menu>
    </Box>
  );
}
