import { useCallback, useEffect, useMemo, useState } from 'react';
import type { SubmitEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  CssBaseline,
  Snackbar,
  TextField,
  ThemeProvider,
  Typography
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import LinkRoundedIcon from '@mui/icons-material/LinkRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import {
  ApiError,
  clearToken,
  createOrganization,
  createShortLink,
  getCurrentUser,
  getLinkAnalytics,
  getOrganizationLinks,
  getOrganization,
  hasSavedToken,
  registerAccount,
  revokeShortLink,
  saveToken,
  signIn,
  updateShortLink
} from './api/api';
import type {
  Organization,
  OrganizationAnalytics,
  ShortLink,
  User
} from './api/api';
import AuthScreen from './components/AuthScreen';
import {
  CenteredLoader,
  CountryClicksChart,
  LinkAnalyticsTable
} from './components/DashboardWidgets';
import Sidebar from './components/Sidebar';
import Header from './components/Header';
import type { DashboardView } from './components/Sidebar';
import LinkTable from './components/LinkTable';
import OrganizationMembers from './components/OrganizationMembers';
import OverviewPanel from './components/OverviewPanel';
import QRCodeDialog from './components/QRCodeDialog';
import WorkspaceDialogs from './components/WorkspaceDialogs';
import theme from './theme';
import { errorMessage } from './utils/errors';
import './App.css';

type Notice = {
  message: string;
  severity: 'success' | 'warning' | 'error' | 'info';
};

const DATA_REFRESH_INTERVAL_MS = 10 * 60 * 1000;

function App() {
  const [token, setToken] = useState(hasSavedToken);
  const [booting, setBooting] = useState(hasSavedToken);
  const [retrySession, setRetrySession] = useState(0);
  const [dataRefreshTick, setDataRefreshTick] = useState(0);
  const [sessionError, setSessionError] = useState('');
  const [user, setUser] = useState<User | null>(null);
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [organizationId, setOrganizationId] = useState<number | ''>('');
  const [links, setLinks] = useState<ShortLink[]>([]);
  const [analytics, setAnalytics] = useState<OrganizationAnalytics | null>(
    null
  );
  const [analyticsRefresh, setAnalyticsRefresh] = useState(0);
  const [analyticsLoading, setAnalyticsLoading] = useState(false);
  const [analyticsError, setAnalyticsError] = useState('');
  const [linksLoading, setLinksLoading] = useState(false);
  const [dataError, setDataError] = useState('');
  const [view, setView] = useState<DashboardView>('overview');
  const [search, setSearch] = useState('');
  const [notice, setNotice] = useState<Notice | null>(null);
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const [organizationDialogOpen, setOrganizationDialogOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<ShortLink | null>(null);
  const [qrTarget, setQrTarget] = useState<ShortLink | null>(null);
  const [linkForm, setLinkForm] = useState({ longUrl: '', expiresAt: '' });
  const [organizationName, setOrganizationName] = useState('');
  const [saving, setSaving] = useState(false);
  const [currentTime, setCurrentTime] = useState(() => Date.now());

  const selectedOrganization = organizations.find(
    (organization) => organization.id === organizationId
  );
  const canManageUsers = Boolean(
    selectedOrganization &&
    (selectedOrganization.owner_user_id === user?.id ||
      selectedOrganization.permissions.includes('organization.manage'))
  );
  const canManageLinks = Boolean(
    selectedOrganization &&
    (selectedOrganization.owner_user_id === user?.id ||
      selectedOrganization.permissions.includes('link.manage'))
  );

  useEffect(() => {
    const handleUnauthorized = () => {
      clearToken();
      setToken(false);
      setBooting(false);
      setUser(null);
      setOrganizations([]);
      setOrganizationId('');
      setLinks([]);
      setLinksLoading(false);
      setAnalytics(null);
      setAnalyticsLoading(false);
      setAnalyticsError('');
      setLinkDialogOpen(false);
      setRevokeTarget(null);
      setView('overview');
      setSessionError('Your session expired. Please sign in again.');
    };
    window.addEventListener('shorty:unauthorized', handleUnauthorized);
    return () =>
      window.removeEventListener('shorty:unauthorized', handleUnauthorized);
  }, []);

  useEffect(() => {
    if (!token) return;
    let active = true;
    Promise.all([getCurrentUser(), getOrganization()])
      .then(([account, orgs]) => {
        if (!active) return;
        setUser(account);
        setOrganizations(orgs);
        setOrganizationId((current) =>
          orgs.some((organization) => organization.id === current)
            ? current
            : (orgs[0]?.id ?? '')
        );
        setLinksLoading(orgs.length > 0);
        setAnalytics(null);
        setAnalyticsLoading(orgs.length > 0);
        setAnalyticsError('');
      })
      .catch((error: unknown) => {
        if (active) setSessionError(errorMessage(error));
      })
      .finally(() => {
        if (active) setBooting(false);
      });
    return () => {
      active = false;
    };
  }, [dataRefreshTick, token, retrySession]);

  useEffect(() => {
    if (!token) return;
    const timer = window.setInterval(() => setCurrentTime(Date.now()), 10_000);
    return () => window.clearInterval(timer);
  }, [token]);

  useEffect(() => {
    if (!token) return;
    const timer = window.setInterval(
      () => setDataRefreshTick((current) => current + 1),
      DATA_REFRESH_INTERVAL_MS
    );
    return () => window.clearInterval(timer);
  }, [token]);

  const loadLinks = useCallback(async () => {
    if (!token || !organizationId) return;
    try {
      setLinks(await getOrganizationLinks(organizationId));
    } catch (error) {
      setDataError(errorMessage(error));
    } finally {
      setLinksLoading(false);
    }
  }, [organizationId, token]);

  useEffect(() => {
    if (!token || !organizationId) return;
    let active = true;
    getOrganizationLinks(organizationId)
      .then((result) => {
        if (active) {
          setLinks(result);
          setDataError('');
        }
      })
      .catch((error: unknown) => {
        if (active) setDataError(errorMessage(error));
      })
      .finally(() => {
        if (active) setLinksLoading(false);
      });
    return () => {
      active = false;
    };
  }, [dataRefreshTick, organizationId, token]);

  useEffect(() => {
    if (!token || !organizationId) {
      return;
    }
    let active = true;
    getLinkAnalytics(organizationId, 'week')
      .then((result) => {
        if (active) {
          setAnalytics(result);
          setAnalyticsError('');
        }
      })
      .catch((error: unknown) => {
        if (active) setAnalyticsError(errorMessage(error));
      })
      .finally(() => {
        if (active) setAnalyticsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [
    analyticsRefresh,
    dataRefreshTick,
    organizationId,
    token
  ]);

  const filteredLinks = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return links;
    return links.filter((link) =>
      [link.short_url, link.long_url, link.short_url_code].some((value) =>
        value.toLowerCase().includes(query)
      )
    );
  }, [links, search]);

  const handleSignIn = async (email: string, password: string) => {
    const response = await signIn(email, password);
    saveToken(response.access_token);
    setToken(true);
    setBooting(true);
    setLinksLoading(true);
    setAnalyticsLoading(true);
    setAnalyticsError('');
    setRetrySession((value) => value + 1);
    setSessionError('');
    setDataError('');
  };

  const handleRegister = async (
    email: string,
    password: string,
    displayName: string
  ) => {
    await registerAccount(email, password, displayName);
    setSessionError('');
  };

  const handleSignOut = () => {
    clearToken();
    setToken(false);
    setBooting(false);
    setUser(null);
    setOrganizations([]);
    setOrganizationId('');
    setLinks([]);
    setLinksLoading(false);
    setAnalytics(null);
    setAnalyticsLoading(false);
    setAnalyticsError('');
    setLinkDialogOpen(false);
    setRevokeTarget(null);
    setView('overview');
    setNotice(null);
  };

  const handleCreateOrganization = async (
    event: SubmitEvent<HTMLFormElement>
  ) => {
    event.preventDefault();
    setSaving(true);
    try {
      const organization = await createOrganization(organizationName.trim());
      setOrganizations((current) => [...current, organization]);
      setOrganizationId(organization.id);
      setLinksLoading(true);
      setDataError('');
      setAnalytics(null);
      setAnalyticsLoading(true);
      setAnalyticsError('');
      setOrganizationName('');
      setOrganizationDialogOpen(false);
      setNotice({ message: 'Workspace created.', severity: 'success' });
    } catch (error) {
      setNotice({ message: errorMessage(error), severity: 'error' });
    } finally {
      setSaving(false);
    }
  };

  const handleCreateLink = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!organizationId || !canManageLinks) return;
    setSaving(true);
    try {
      const result = await createShortLink(
        organizationId,
        linkForm.longUrl.trim(),
        linkForm.expiresAt
          ? new Date(linkForm.expiresAt).toISOString()
          : undefined
      );
      setLinkDialogOpen(false);
      setLinkForm({ longUrl: '', expiresAt: '' });
      setLinksLoading(true);
      setDataError('');
      await loadLinks();
      setAnalyticsLoading(true);
      setAnalyticsError('');
      setAnalyticsRefresh((value) => value + 1);
      setNotice(
        result.reused
          ? {
              message:
                'A link for this URL already exists. Reusing the existing short link.',
              severity: 'warning'
            }
          : { message: 'Your short link is ready.', severity: 'success' }
      );
    } catch (error) {
      setNotice({
        message: errorMessage(error),
        severity:
          error instanceof ApiError && error.status === 409
            ? 'warning'
            : 'error'
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      setNotice({
        message: 'Short link copied to clipboard.',
        severity: 'success'
      });
    } catch {
      setNotice({
        message:
          'Could not access the clipboard. Copy the link directly instead.',
        severity: 'error'
      });
    }
  };

  const handleQRCode = (link: ShortLink) => {
    setQrTarget(link);
  };

  const handleQRCodeError = useCallback((message: string) => {
    setNotice({ message, severity: 'error' });
  }, []);

  const handleToggleLink = async (link: ShortLink) => {
    if (!canManageLinks) return;
    try {
      const nextStatus = link.status === 'active' ? 'disabled' : 'active';
      const updated = await updateShortLink(link.short_url_code, nextStatus);
      setLinks((current) =>
        current.map((item) =>
          item.id === link.id ? { ...item, status: updated.status } : item
        )
      );
      setNotice({
        message: `Link ${nextStatus === 'active' ? 'enabled' : 'disabled'}.`,
        severity: 'success'
      });
    } catch (error) {
      setNotice({ message: errorMessage(error), severity: 'error' });
    }
  };

  const handleRevokeLink = async () => {
    if (!revokeTarget || !canManageLinks) return;
    try {
      await revokeShortLink(revokeTarget.short_url_code);
      setLinks((current) =>
        current.map((item) =>
          item.id === revokeTarget.id ? { ...item, status: 'revoked' } : item
        )
      );
      setNotice({
        message: 'Link revoked. It can no longer be reactivated.',
        severity: 'success'
      });
      setRevokeTarget(null);
    } catch (error) {
      setNotice({ message: errorMessage(error), severity: 'error' });
    }
  };

  if (booting) {
    return (
      <Box className="loading-screen">
        <CircularProgress />
        <Typography color="text.secondary">Loading your workspace…</Typography>
      </Box>
    );
  }

  if (!token || !user) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AuthScreen
          onSignIn={handleSignIn}
          onRegister={handleRegister}
          sessionError={sessionError}
        />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box className="dashboard-shell">
        <Sidebar
          user={user}
          view={view}
          linkCount={links.length}
          canManageUsers={canManageUsers}
          onViewChange={setView}
          onSignOut={handleSignOut}
        />

        <Box component="main" className="dashboard-main">
          <Header
            view={view}
            organizationName={selectedOrganization?.name ?? ''}
            canManageLinks={canManageLinks}
            onCreateWorkspace={() => setOrganizationDialogOpen(true)}
            onCreateLink={() => setLinkDialogOpen(true)}
          />

          <Box className="page-content">
            {dataError && (
              <Alert
                severity="error"
                action={
                  <Button
                    color="inherit"
                    size="small"
                    onClick={() => {
                      setBooting(true);
                      setLinksLoading(true);
                      setDataError('');
                      setRetrySession((value) => value + 1);
                      void loadLinks();
                    }}
                  >
                    Retry
                  </Button>
                }
                sx={{ mb: 2 }}
              >
                {dataError}
              </Alert>
            )}
            {organizations.length > 0 && !canManageLinks && (
              <Alert severity="info" sx={{ mb: 2 }}>
                You can view this workspace&apos;s links and analytics. The
                link.manage permission is required to create or change links.
              </Alert>
            )}

            {organizations.length === 0 ? (
              <Card className="onboarding-card">
                <CardContent>
                  <span className="empty-icon">
                    <LinkRoundedIcon className="link-icon" />
                  </span>
                  <Typography variant="h5" sx={{ mt: 2, fontWeight: 750 }}>
                    Create your first workspace
                  </Typography>
                  <Typography color="text.secondary" sx={{ mt: 1, mb: 3 }}>
                    A workspace keeps your Shorty links and click stats
                    together.
                  </Typography>
                  <Button
                    variant="contained"
                    startIcon={<AddRoundedIcon />}
                    onClick={() => setOrganizationDialogOpen(true)}
                  >
                    Create workspace
                  </Button>
                </CardContent>
              </Card>
            ) : (
              <>
                {view === 'overview' && (
                  <OverviewPanel
                    userName={user.display_name || user.email.split('@')[0]}
                    canManageLinks={canManageLinks}
                    links={links}
                    linksLoading={linksLoading}
                    currentTime={currentTime}
                    onCreateLink={() => setLinkDialogOpen(true)}
                    onViewAllLinks={() => setView('links')}
                    onCopy={handleCopy}
                    onQRCode={handleQRCode}
                    onToggle={handleToggleLink}
                    onRevoke={setRevokeTarget}
                  />
                )}
                {view === 'links' && (
                  <Card className="content-card links-page-card">
                    <CardContent>
                      <Box className="section-heading">
                        <Box>
                          <Typography variant="h6" sx={{ fontWeight: 750 }}>
                            All links
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            {links.length} links in this workspace
                          </Typography>
                        </Box>
                        <TextField
                          className="search-field"
                          placeholder="Search links"
                          value={search}
                          onChange={(event) => setSearch(event.target.value)}
                          slotProps={{
                            input: {
                              startAdornment: (
                                <SearchRoundedIcon
                                  fontSize="small"
                                  sx={{ mr: 1, color: 'text.secondary' }}
                                />
                              )
                            }
                          }}
                        />
                      </Box>
                      {linksLoading ? (
                        <CenteredLoader />
                      ) : (
                        <LinkTable
                          canManageLinks={canManageLinks}
                          links={filteredLinks}
                          currentTime={currentTime}
                          onCopy={handleCopy}
                          onQRCode={handleQRCode}
                          onToggle={handleToggleLink}
                          onRevoke={setRevokeTarget}
                        />
                      )}
                    </CardContent>
                  </Card>
                )}
                {view === 'analytics' && (
                  <>
                    {!analyticsLoading && !analyticsError && (
                      <CountryClicksChart
                        countries={analytics?.countries ?? []}
                      />
                    )}
                    <LinkAnalyticsTable
                      organizationId={organizationId}
                      links={links}
                      loading={analyticsLoading}
                      error={analyticsError}
                    />
                  </>
                )}
                {view === 'members' && canManageUsers && organizationId && (
                  <OrganizationMembers
                    key={organizationId}
                    organizationId={organizationId}
                    currentUserId={user.id}
                    refreshKey={dataRefreshTick}
                    onNotice={(message, severity) =>
                      setNotice({ message, severity })
                    }
                  />
                )}
              </>
            )}
          </Box>
          <Typography className="footer-note">
            Shorty · Less URL. More impact.
          </Typography>
        </Box>
      </Box>
      <WorkspaceDialogs
        linkDialogOpen={linkDialogOpen}
        organizationDialogOpen={organizationDialogOpen}
        revokeTarget={revokeTarget}
        canManageLinks={canManageLinks}
        saving={saving}
        longUrl={linkForm.longUrl}
        expiresAt={linkForm.expiresAt}
        organizationName={organizationName}
        onLinkDialogClose={() => !saving && setLinkDialogOpen(false)}
        onOrganizationDialogClose={() =>
          !saving && setOrganizationDialogOpen(false)
        }
        onRevokeDialogClose={() => setRevokeTarget(null)}
        onCreateLink={handleCreateLink}
        onCreateOrganization={handleCreateOrganization}
        onConfirmRevoke={() => void handleRevokeLink()}
        onLongUrlChange={(longUrl) =>
          setLinkForm((current) => ({ ...current, longUrl }))
        }
        onExpiresAtChange={(expiresAt) =>
          setLinkForm((current) => ({ ...current, expiresAt }))
        }
        onOrganizationNameChange={setOrganizationName}
      />
      {qrTarget && (
        <QRCodeDialog
          key={qrTarget.short_url_code}
          link={qrTarget}
          onClose={() => setQrTarget(null)}
          onError={handleQRCodeError}
        />
      )}
      <Snackbar
        open={Boolean(notice)}
        autoHideDuration={5000}
        onClose={() => setNotice(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        {notice ? (
          <Alert
            onClose={() => setNotice(null)}
            severity={notice.severity}
            variant="filled"
          >
            {notice.message}
          </Alert>
        ) : undefined}
      </Snackbar>
    </ThemeProvider>
  );
}

export default App;
