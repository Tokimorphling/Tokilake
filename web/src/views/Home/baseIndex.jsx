import { Box, Typography, Button, Container, Stack, useTheme, alpha } from '@mui/material';
import Grid from '@mui/material/Unstable_Grid2';
import { GitHub } from '@mui/icons-material';
import { useTranslation } from 'react-i18next';
import TokilakeAnimation from './TokilakeAnimation';

const BaseIndex = () => {
  const { t } = useTranslation();
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';

  return (
    <Box
      sx={{
        minHeight: 'calc(100vh - 136px)',
        background: isDark
          ? `radial-gradient(circle at 2% 10%, ${alpha(theme.palette.primary.main, 0.15)} 0%, transparent 40%), 
             radial-gradient(circle at 95% 20%, ${alpha(theme.palette.secondary.main, 0.1)} 0%, transparent 40%),
             linear-gradient(${alpha(theme.palette.background.default, 0.9)}, ${alpha(theme.palette.background.default, 0.9)}),
             url("data:image/svg+xml,%3Csvg width='20' height='20' viewBox='0 0 20 20' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='%239C92AC' fill-opacity='0.05' fill-rule='evenodd'%3E%3Ccircle cx='3' cy='3' r='1'/%3E%3C/g%3E%3C/svg%3E"),
             ${theme.palette.background.default}`
          : `radial-gradient(circle at 2% 10%, ${alpha(theme.palette.primary.main, 0.1)} 0%, transparent 40%), 
             radial-gradient(circle at 95% 20%, ${alpha(theme.palette.secondary.main, 0.08)} 0%, transparent 40%),
             linear-gradient(${alpha(theme.palette.background.paper, 0.8)}, ${alpha(theme.palette.background.paper, 0.8)}),
             url("data:image/svg+xml,%3Csvg width='20' height='20' viewBox='0 0 20 20' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='%23000000' fill-opacity='0.03' fill-rule='evenodd'%3E%3Ccircle cx='3' cy='3' r='1'/%3E%3C/g%3E%3C/svg%3E"),
             ${theme.palette.grey[50]}`,
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        overflow: 'hidden'
      }}
    >
      <Container maxWidth="lg" sx={{ zIndex: 1 }}>
        <Grid container spacing={4} alignItems="center">
          <Grid xs={12} md={7} lg={6}>
            <Stack spacing={4}>
              <Stack spacing={1}>
                <Typography
                  variant="h1"
                  sx={{
                    fontSize: { xs: '3rem', md: '4.5rem' },
                    fontWeight: 800,
                    background: `linear-gradient(45deg, ${theme.palette.primary.main}, ${theme.palette.secondary.main})`,
                    WebkitBackgroundClip: 'text',
                    WebkitTextFillColor: 'transparent',
                    lineHeight: 1.2
                  }}
                >
                  Tokilake
                </Typography>
                <Typography
                  variant="h2"
                  sx={{
                    fontSize: { xs: '1.2rem', md: '1.5rem' },
                    color: theme.palette.text.secondary,
                    fontWeight: 500,
                    lineHeight: 1.6,
                    maxWidth: '500px'
                  }}
                >
                  {t('description')}
                </Typography>
              </Stack>
              <Stack direction="row" spacing={2}>
                <Button
                  variant="contained"
                  size="large"
                  startIcon={<GitHub />}
                  href="https://github.com/MartialBE/one-hub"
                  target="_blank"
                  sx={{
                    borderRadius: '12px',
                    px: 4,
                    py: 1.5,
                    boxShadow: `0 8px 16px ${alpha(theme.palette.primary.main, 0.2)}`
                  }}
                >
                  GitHub
                </Button>
                <Button
                  variant="outlined"
                  size="large"
                  href="/login"
                  sx={{
                    borderRadius: '12px',
                    px: 4,
                    borderColor: theme.palette.divider,
                    color: theme.palette.text.primary,
                    '&:hover': {
                      borderColor: theme.palette.primary.main,
                      backgroundColor: alpha(theme.palette.primary.main, 0.04)
                    }
                  }}
                >
                  {t('home.start_now') === 'home.start_now' ? 'Start Now' : t('home.start_now')}
                </Button>
              </Stack>
            </Stack>
          </Grid>
          <Grid xs={12} md={5} lg={6} sx={{ display: { xs: 'none', md: 'block' } }}>
            <TokilakeAnimation />
          </Grid>
        </Grid>
      </Container>
    </Box>
  );
};

export default BaseIndex;
