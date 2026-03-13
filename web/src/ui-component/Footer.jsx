// material-ui
import { Link, Container, Box } from '@mui/material';
import React from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

// ==============================|| FOOTER - AUTHENTICATION 2 & 3 ||============================== //

const Footer = () => {
  const siteInfo = useSelector((state) => state.siteInfo);
  const { t } = useTranslation();

  return (
    <Container sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '64px', borderRadius: 0 }}>
      <Box sx={{ textAlign: 'center' }}>
        {siteInfo.footer_html ? (
          <div className="custom-footer" dangerouslySetInnerHTML={{ __html: siteInfo.footer_html }}></div>
        ) : (
          <>
            <Box component="span" sx={{ display: 'block', fontSize: '12px', mt: 0.5, opacity: 0.6 }}>
              {t('footer.poweredBy')}{' '}
              <Link href="https://github.com/MartialBE/one-hub" target="_blank" color="inherit">
                One Hub
              </Link>
            </Box>
          </>
        )}
      </Box>
    </Container>
  );
};

export default Footer;
