import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography
} from '@mui/material';
import type { SubmitEvent } from 'react';
import type { ShortLink } from '../api/api';

export default function WorkspaceDialogs({
  linkDialogOpen,
  organizationDialogOpen,
  revokeTarget,
  canManageLinks,
  saving,
  longUrl,
  expiresAt,
  organizationName,
  onLinkDialogClose,
  onOrganizationDialogClose,
  onRevokeDialogClose,
  onCreateLink,
  onCreateOrganization,
  onConfirmRevoke,
  onLongUrlChange,
  onExpiresAtChange,
  onOrganizationNameChange
}: {
  linkDialogOpen: boolean;
  organizationDialogOpen: boolean;
  revokeTarget: ShortLink | null;
  canManageLinks: boolean;
  saving: boolean;
  longUrl: string;
  expiresAt: string;
  organizationName: string;
  onLinkDialogClose: () => void;
  onOrganizationDialogClose: () => void;
  onRevokeDialogClose: () => void;
  onCreateLink: (event: SubmitEvent<HTMLFormElement>) => void;
  onCreateOrganization: (event: SubmitEvent<HTMLFormElement>) => void;
  onConfirmRevoke: () => void;
  onLongUrlChange: (value: string) => void;
  onExpiresAtChange: (value: string) => void;
  onOrganizationNameChange: (value: string) => void;
}) {
  return (
    <>
      <Dialog
        open={linkDialogOpen && canManageLinks}
        onClose={onLinkDialogClose}
        fullWidth
        maxWidth="sm"
      >
        <Box component="form" onSubmit={onCreateLink}>
          <DialogTitle sx={{ fontWeight: 750 }}>
            Create a short link
          </DialogTitle>
          <DialogContent className="dialog-fields">
            <Typography variant="body2" color="text.secondary">
              Paste the long URL you want to make easier to share.
            </Typography>
            <TextField
              label="Destination URL"
              type="url"
              placeholder="https://example.com/your/long/url"
              value={longUrl}
              onChange={(event) => onLongUrlChange(event.target.value)}
              autoFocus
              required
              fullWidth
            />
            <TextField
              label="Expiration date (optional)"
              type="datetime-local"
              value={expiresAt}
              onChange={(event) => onExpiresAtChange(event.target.value)}
              slotProps={{ inputLabel: { shrink: true } }}
              fullWidth
            />
            {/* <Typography variant="caption" color="text.secondary">
              Expiration uses your browser&apos;s local time zone (
              {Intl.DateTimeFormat().resolvedOptions().timeZone}).
            </Typography> */}
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2.5 }}>
            <Button onClick={onLinkDialogClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" variant="contained" disabled={saving}>
              {saving ? (
                <CircularProgress size={20} color="inherit" />
              ) : (
                'Create link'
              )}
            </Button>
          </DialogActions>
        </Box>
      </Dialog>

      <Dialog
        open={organizationDialogOpen}
        onClose={onOrganizationDialogClose}
        fullWidth
        maxWidth="xs"
      >
        <Box component="form" onSubmit={onCreateOrganization}>
          <DialogTitle sx={{ fontWeight: 750 }}>New workspace</DialogTitle>
          <DialogContent className="dialog-fields">
            <Typography variant="body2" color="text.secondary">
              Give this group of links a name.
            </Typography>
            <TextField
              label="Workspace name"
              value={organizationName}
              onChange={(event) => onOrganizationNameChange(event.target.value)}
              autoFocus
              required
              fullWidth
            />
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2.5 }}>
            <Button onClick={onOrganizationDialogClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" variant="contained" disabled={saving}>
              {saving ? (
                <CircularProgress size={20} color="inherit" />
              ) : (
                'Create workspace'
              )}
            </Button>
          </DialogActions>
        </Box>
      </Dialog>

      <Dialog
        open={Boolean(revokeTarget && canManageLinks)}
        onClose={onRevokeDialogClose}
      >
        <DialogTitle sx={{ fontWeight: 750 }}>Revoke this link?</DialogTitle>
        <DialogContent>
          <Typography color="text.secondary">
            Anyone using this short URL will no longer be redirected. Revoked
            links cannot be reactivated.
          </Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2.5 }}>
          <Button onClick={onRevokeDialogClose}>Cancel</Button>
          <Button color="error" variant="contained" onClick={onConfirmRevoke}>
            Revoke link
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
