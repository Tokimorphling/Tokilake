import { useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useSelector } from 'react-redux';

// material-ui
import { useTheme } from '@mui/material/styles';
import { Box, Button, Divider, Grid, Stack, Typography, useMediaQuery } from '@mui/material';

// project imports
import AuthWrapper from '../AuthWrapper';
import AuthCardWrapper from '../AuthCardWrapper';
import Logo from 'ui-component/Logo';
import AuthRegister from '../AuthForms/AuthRegister';

// assets
import Google from 'assets/images/icons/social-google.svg';
import { useTranslation } from 'react-i18next';
import { onGoogleOAuthClicked } from 'utils/common';

// ===============================|| AUTH3 - REGISTER ||=============================== //

const Register = () => {
  const { t } = useTranslation();
  const theme = useTheme();
  const matchDownSM = useMediaQuery(theme.breakpoints.down('md'));
  const siteInfo = useSelector((state) => state.siteInfo);
  const [searchParams] = useSearchParams();
  const showPasswordRegister = !siteInfo.isLoading && siteInfo.register_enabled && siteInfo.password_register;
  const showGoogleRegister = !siteInfo.isLoading && siteInfo.google_oauth && siteInfo.google_only_register;

  useEffect(() => {
    const affCode = searchParams.get('aff');
    if (affCode) {
      localStorage.setItem('aff', affCode);
    }
  }, [searchParams]);

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
                            {t('menu.signup')}
                          </Typography>
                        </Stack>
                      </Grid>
                    </Grid>
                  </Grid>
                  <Grid item xs={12}>
                    {siteInfo.isLoading ? null : showPasswordRegister ? (
                      <AuthRegister />
                    ) : showGoogleRegister ? (
                      <Button
                        fullWidth
                        size="large"
                        variant="outlined"
                        onClick={() => onGoogleOAuthClicked()}
                        sx={{
                          ...theme.typography.LoginButton
                        }}
                      >
                        <Box sx={{ mr: { xs: 1, sm: 2, width: 20 }, display: 'flex', alignItems: 'center' }}>
                          <img src={Google} alt="Google" width={25} height={25} style={{ marginRight: matchDownSM ? 8 : 16 }} />
                        </Box>
                        {t('registerPage.googleOnlyRegister')}
                      </Button>
                    ) : (
                      <Typography variant="body1" color="textSecondary" align="center">
                        {t('registerPage.passwordRegisterDisabled')}
                      </Typography>
                    )}
                  </Grid>
                  <Grid item xs={12}>
                    <Divider />
                  </Grid>
                  <Grid item xs={12}>
                    <Grid item container direction="column" alignItems="center" xs={12}>
                      <Typography component={Link} to="/login" variant="subtitle1" sx={{ textDecoration: 'none' }}>
                        {t('registerPage.alreadyHaveAccount')}
                      </Typography>
                    </Grid>
                  </Grid>
                </Grid>
              </AuthCardWrapper>
            </Grid>
          </Grid>
        </Grid>
        {/* <Grid item xs={12} sx={{ m: 3, mt: 1 }}>
          <AuthFooter />
        </Grid> */}
      </Grid>
    </AuthWrapper>
  );
};

export default Register;
