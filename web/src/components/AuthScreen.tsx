import { useState } from 'react';
import type { SubmitEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  Divider,
  TextField,
  Typography
} from '@mui/material';
import { errorMessage } from '../utils/errors';

export default function AuthScreen({
  onSignIn,
  onRegister,
  sessionError
}: {
  onSignIn: (email: string, password: string) => Promise<void>;
  onRegister: (
    email: string,
    password: string,
    displayName: string
  ) => Promise<void>;
  sessionError: string;
}) {
  const [registering, setRegistering] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const submit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    setSuccess('');
    try {
      if (registering) {
        await onRegister(email, password, displayName);
        setSuccess(
          'Account created. Check your email for the verification link, then sign in.'
        );
        setRegistering(false);
        setPassword('');
      } else {
        await onSignIn(email, password);
      }
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box className="auth-page">
      <Box className="auth-decoration auth-decoration-one" />
      <Box className="auth-decoration auth-decoration-two" />
      <Card className="auth-card">
        <Box className="auth-brand">
          <span className="brand-mark">
            <img src="/favicon.svg" width={38} height={38} alt="" />
          </span>
          <Typography variant="h5" sx={{ fontWeight: 800 }}>
            shorty
          </Typography>
        </Box>
        <Typography
          variant="h4"
          sx={{ fontWeight: 750 }}
          className="auth-heading"
        >
          {registering ? 'Create your account' : 'Welcome back'}
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 3 }}>
          {registering
            ? 'Sign up to start organizing and sharing your links.'
            : 'Sign in to manage your links and see how they perform.'}
        </Typography>
        {sessionError && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {sessionError}
          </Alert>
        )}
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}
        {success && (
          <Alert severity="success" sx={{ mb: 2 }}>
            {success}
          </Alert>
        )}
        <Box component="form" onSubmit={submit} className="auth-form">
          {registering && (
            <TextField
              label="Display name"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              autoComplete="name"
              required
              fullWidth
            />
          )}
          <TextField
            label="Email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            autoComplete="email"
            required
            fullWidth
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete={registering ? 'new-password' : 'current-password'}
            slotProps={{
              htmlInput: registering
                ? { minLength: 12, maxLength: 30 }
                : { maxLength: 30 }
            }}
            helperText={registering ? 'Use at least 12 characters.' : undefined}
            required
            fullWidth
          />
          <Button
            type="submit"
            variant="contained"
            size="large"
            disabled={busy}
            fullWidth
          >
            {busy ? (
              <CircularProgress size={22} color="inherit" />
            ) : registering ? (
              'Create account'
            ) : (
              'Sign in'
            )}
          </Button>
        </Box>
        <Divider sx={{ my: 2.5 }} />
        <Typography
          sx={{ textAlign: 'center' }}
          color="text.secondary"
          variant="body2"
        >
          {registering ? 'Already have an account? ' : 'New to Shorty? '}
          <Button
            size="small"
            onClick={() => {
              setRegistering(!registering);
              setError('');
              setSuccess('');
            }}
          >
            {registering ? 'Sign in' : 'Create an account'}
          </Button>
        </Typography>
      </Card>
    </Box>
  );
}
