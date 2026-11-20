import { createTheme } from '@mui/material';

const theme = createTheme({
  palette: {
    primary: { main: '#5747e8' },
    secondary: { main: '#26b5a5' },
    background: { default: '#f5f6fa', paper: '#ffffff' },
    text: { primary: '#1e2235', secondary: '#777d91' }
  },
  shape: { borderRadius: 14 },
  typography: {
    fontFamily: "'Inter', 'Segoe UI', sans-serif",
    button: { textTransform: 'none', fontWeight: 650 }
  },
  components: {
    MuiCard: {
      styleOverrides: {
        root: {
          border: '1px solid #eceef4',
          boxShadow: '0 8px 26px rgba(32, 38, 70, 0.045)'
        }
      }
    },
    MuiButton: {
      styleOverrides: { root: { borderRadius: 10, paddingInline: 16 } }
    },
    MuiTextField: { defaultProps: { size: 'small' } }
  }
});

export default theme;
