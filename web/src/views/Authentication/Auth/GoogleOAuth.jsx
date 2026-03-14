import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import React, { useEffect, useState } from 'react';
import { showError } from 'utils/common';
import useLogin from 'hooks/useLogin';

import { useTheme } from '@mui/material/styles';
import { Grid, Stack, Typography, useMediaQuery, CircularProgress } from '@mui/material';

import AuthWrapper from '../AuthWrapper';
import AuthCardWrapper from '../AuthCardWrapper';
import Logo from 'ui-component/Logo';
import { useTranslation } from 'react-i18next';

const GoogleOAuth = () => {
  const { t } = useTranslation();
  const theme = useTheme();
  const matchDownSM = useMediaQuery(theme.breakpoints.down('md'));

  const [searchParams] = useSearchParams();
  const [prompt, setPrompt] = useState(t('common.processing'));
  const { googleLogin } = useLogin();

  const navigate = useNavigate();

  const sendCode = async (code, state, count) => {
    const { success, message } = await googleLogin(code, state);
    if (!success) {
      if (message) {
        showError(message);
      }
      if (count === 0) {
        setPrompt(t('login.googleError'));
        await new Promise((resolve) => setTimeout(resolve, 2000));
        navigate('/login');
        return;
      }
      count++;
      setPrompt(t('login.googleCountError', { count }));
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await sendCode(code, state, count);
    }
  };

  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    sendCode(code, state, 0).then();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <AuthWrapper>
      <Grid container direction="column" justifyContent="flex-end">
        <Grid item xs={12}>
          <Grid container justifyContent="center" alignItems="center" sx={{ minHeight: 'calc(100vh - 136px)' }}>
            <Grid item sx={{ m: { xs: 1, sm: 3 }, mb: 0 }}>
              <AuthCardWrapper>
                <Grid container spacing={2} alignItems="center" justifyContent="center">
                  <Grid item sx={{ mb: 3 }}>
                    <Link to="#">
                      <Logo />
                    </Link>
                  </Grid>
                  <Grid item xs={12}>
                    <Grid container direction={matchDownSM ? 'column-reverse' : 'row'} alignItems="center" justifyContent="center">
                      <Grid item>
                        <Stack alignItems="center" justifyContent="center" spacing={1}>
                          <Typography color={theme.palette.primary.main} gutterBottom variant={matchDownSM ? 'h3' : 'h2'}>
                            {t('login.googleLogin')}
                          </Typography>
                        </Stack>
                      </Grid>
                    </Grid>
                  </Grid>
                  <Grid item xs={12} container direction="column" justifyContent="center" alignItems="center" style={{ height: '200px' }}>
                    <CircularProgress />
                    <Typography variant="h3" paddingTop={'20px'}>
                      {prompt}
                    </Typography>
                  </Grid>
                </Grid>
              </AuthCardWrapper>
            </Grid>
          </Grid>
        </Grid>
      </Grid>
    </AuthWrapper>
  );
};

export default GoogleOAuth;
