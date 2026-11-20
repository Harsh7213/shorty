import { Box, Button, Typography } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';

export type DashboardView = 'overview' | 'links' | 'analytics' | 'members';

export default function Header({
  view,
  organizationName,
  canManageLinks,
  onCreateWorkspace,
  onCreateLink
}: {
  view: DashboardView;
  organizationName: string;
  canManageLinks: boolean;
  onCreateWorkspace: () => void;
  onCreateLink: () => void;
}) {
  const viewLabel =
    view === 'overview'
      ? 'Overview'
      : view === 'links'
        ? 'My links'
        : view === 'analytics'
          ? 'Link analytics'
          : 'User management';

  return (
    <header className="topbar">
      <Box>
        <Typography variant="body2" color="text.secondary">
          Workspace / {viewLabel}
        </Typography>
        <Typography variant="h5" sx={{ fontWeight: 750 }}>
          {view === 'overview' ? 'Dashboard' : viewLabel}
        </Typography>
      </Box>
      <Box className="topbar-actions">
        {organizationName ? (
          <Typography
            className="organization-name"
            variant="subtitle1"
            sx={{
              color: 'text.primary',
              fontWeight: 700,
              letterSpacing: '-0.02em'
            }}
            title={organizationName}
          >
            {organizationName}
          </Typography>
        ) : (
          <Button variant="outlined" onClick={onCreateWorkspace}>
            New workspace
          </Button>
        )}
        {canManageLinks && (
          <Button
            variant="contained"
            startIcon={<AddRoundedIcon />}
            onClick={onCreateLink}
          >
            Create link
          </Button>
        )}
      </Box>
    </header>
  );
}
