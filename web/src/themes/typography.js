/**
 * @param {JsonObject} theme theme customization object
 */

const primaryFont =
  '"Inter", "Public Sans Variable", -apple-system, BlinkMacSystemFont, sans-serif';
const secondaryFont =
  '"Outfit", "Inter", -apple-system, BlinkMacSystemFont, sans-serif';

export default function themeTypography(theme) {
  return {
    fontFamily: primaryFont,
    fontSecondaryFamily: secondaryFont,
    fontWeightLight: 300,
    fontWeightRegular: 400,
    fontWeightMedium: 500,
    fontWeightSemiBold: 600,
    fontWeightBold: 700,
    h1: {
      fontWeight: 700,
      fontSize: '2.25rem',
      lineHeight: 1.2,
      fontFamily: secondaryFont,
      color: theme.heading,
      letterSpacing: '-0.5px'
    },
    h2: {
      fontWeight: 700,
      fontSize: '1.75rem',
      lineHeight: 1.2,
      fontFamily: secondaryFont,
      color: theme.heading,
      letterSpacing: '-0.3px'
    },
    h3: {
      fontWeight: 700,
      fontSize: '1.5rem',
      lineHeight: 1.3,
      fontFamily: secondaryFont,
      color: theme.heading
    },
    h4: {
      fontWeight: 600,
      fontSize: '1.25rem',
      lineHeight: 1.4,
      fontFamily: secondaryFont,
      color: theme.heading
    },
    h5: {
      fontWeight: 600,
      fontSize: '1rem',
      lineHeight: 1.5,
      fontFamily: secondaryFont,
      color: theme.heading
    },
    h6: {
      fontWeight: 600,
      fontSize: '0.875rem',
      lineHeight: 1.5,
      fontFamily: secondaryFont,
      color: theme.heading
    },
    subtitle1: {
      fontSize: '1rem',
      lineHeight: 1.5,
      fontWeight: 600,
      color: theme.textDark
    },
    subtitle2: {
      fontSize: '0.875rem',
      lineHeight: 1.5,
      fontWeight: 500,
      color: theme.darkTextSecondary
    },
    body1: {
      fontSize: '1rem',
      lineHeight: 1.6,
      color: theme.darkTextPrimary,
      fontFamily: primaryFont
    },
    body2: {
      fontSize: '0.875rem',
      lineHeight: 1.6,
      color: theme.darkTextPrimary,
      fontFamily: primaryFont
    },
    caption: {
      fontSize: '0.75rem',
      lineHeight: 1.5,
      color: theme.darkTextSecondary
    },
    overline: {
      fontSize: '0.75rem',
      fontWeight: 700,
      lineHeight: 1.5,
      textTransform: 'uppercase',
      color: theme.darkTextSecondary,
      letterSpacing: '1px'
    },
    button: {
      textTransform: 'none',
      fontWeight: 600,
      fontSize: '0.875rem',
      lineHeight: 1.5
    },
    customInput: {
      marginTop: 1,
      marginBottom: 1,
      '& > label': {
        top: 23,
        left: 0,
        color: theme.grey500,
        '&[data-shrink="false"]': {
          top: 5
        }
      },
      '& > div > input': {
        padding: '30.5px 14px 11.5px !important'
      },
      '& legend': {
        display: 'none'
      },
      '& fieldset': {
        top: 0
      }
    },
    otherInput: {
      marginTop: 1,
      marginBottom: 1
    },
    mainContent: {
      backgroundColor: theme.background,
      width: '100%',
      minHeight: 'calc(100vh - 88px)',
      flexGrow: 1,
      padding: '24px',
      paddingBottom: '40px',
      marginTop: '88px',
      marginRight: '0',
      marginBottom: '20px',
      borderRadius: '0',
      position: 'relative'
    },
    menuCaption: {
      fontSize: '0.75rem',
      fontWeight: 700,
      color: theme.colors?.grey500,
      padding: '6px',
      textTransform: 'uppercase',
      letterSpacing: '1px',
      marginTop: '16px'
    },
    subMenuCaption: {
      fontSize: '0.75rem',
      fontWeight: 500,
      color: theme.darkTextSecondary,
      textTransform: 'capitalize'
    },
    commonAvatar: {
      cursor: 'pointer',
      borderRadius: '4px'
    },
    smallAvatar: {
      width: '24px',
      height: '24px',
      fontSize: '0.875rem'
    },
    mediumAvatar: {
      width: '40px',
      height: '40px',
      fontSize: '1.2rem'
    },
    largeAvatar: {
      width: '40px',
      height: '40px',
      fontSize: '1.25rem'
    },
    menuButton: {
      color: theme.menuButtonColor,
      background: theme.menuButton
    },
    menuChip: {
      background: theme.menuChip
    },
    CardWrapper: {
      backgroundColor: theme.mode === 'dark' ? theme.colors.darkLevel2 : theme.colors.primaryDark
    },
    SubCard: {
      border: theme.mode === 'dark' ? '1px solid rgba(227, 232, 239, 0.2)' : '1px solid rgb(227, 232, 239)'
    },
    LoginButton: {
      color: theme.darkTextPrimary,
      backgroundColor: theme.mode === 'dark' ? theme.backgroundDefault : theme.colors?.grey50,
      borderColor: theme.mode === 'dark' ? theme.colors?.grey700 : theme.colors?.grey100
    }
  };
}
