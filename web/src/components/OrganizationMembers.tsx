import { useCallback, useEffect, useMemo, useState } from 'react';
import type { SubmitEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormGroup,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import GroupsRoundedIcon from '@mui/icons-material/GroupsRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import {
  getOrganizationMembers,
  removeOrganizationMember,
  saveOrganizationMember
} from '../api/api';
import type { OrganizationMember, OrganizationPermission } from '../api/api';
import { errorMessage } from '../utils/errors';

type MemberForm = {
  email: string;
  displayName: string;
  password: string;
  permissions: OrganizationPermission[];
};

type NoticeSeverity = 'success' | 'error' | 'info';

const emptyMemberForm = (): MemberForm => ({
  email: '',
  displayName: '',
  password: '',
  permissions: []
});

const permissionDescriptions: Record<
  OrganizationPermission,
  { title: string; description: string }
> = {
  'link.manage': {
    title: 'Manage links',
    description: 'Create, update, and revoke workspace links.'
  },
  'organization.manage': {
    title: 'Manage organization',
    description: 'Add members, assign permissions, and remove members.'
  }
};

const organizationPermissions: OrganizationPermission[] = [
  'link.manage',
  'organization.manage'
];

function formatJoinedDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  }).format(new Date(value));
}

export default function OrganizationMembers({
  organizationId,
  currentUserId,
  refreshKey,
  onNotice
}: {
  organizationId: number;
  currentUserId: number;
  refreshKey: number;
  onNotice: (message: string, severity: NoticeSeverity) => void;
}) {
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingMember, setEditingMember] = useState<OrganizationMember | null>(
    null
  );
  const [removeTarget, setRemoveTarget] = useState<OrganizationMember | null>(
    null
  );
  const [form, setForm] = useState<MemberForm>(emptyMemberForm);

  const loadMembers = useCallback(async () => {
    try {
      setMembers(await getOrganizationMembers(organizationId));
      setError('');
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, [organizationId]);

  useEffect(() => {
    let active = true;
    getOrganizationMembers(organizationId)
      .then((result) => {
        if (!active) return;
        setMembers(result);
        setError('');
      })
      .catch((loadError: unknown) => {
        if (active) setError(errorMessage(loadError));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [loadMembers, organizationId, refreshKey]);

  const refreshMembers = async () => {
    setLoading(true);
    setError('');
    await loadMembers();
  };

  const filteredMembers = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return members;
    return members.filter((member) =>
      [member.display_name, member.email].some((value) =>
        value.toLowerCase().includes(query)
      )
    );
  }, [members, search]);

  const openAddDialog = () => {
    setEditingMember(null);
    setForm(emptyMemberForm());
    setDialogOpen(true);
  };

  const openEditDialog = (member: OrganizationMember) => {
    setEditingMember(member);
    setForm({
      email: member.email,
      displayName: member.display_name,
      password: '',
      permissions: member.permissions
    });
    setDialogOpen(true);
  };

  const closeMemberDialog = () => {
    if (saving) return;
    setDialogOpen(false);
    setEditingMember(null);
  };

  const togglePermission = (permission: OrganizationPermission) => {
    setForm((current) => ({
      ...current,
      permissions: current.permissions.includes(permission)
        ? current.permissions.filter((value) => value !== permission)
        : [...current.permissions, permission]
    }));
  };

  const handleSaveMember = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    try {
      const result = await saveOrganizationMember(organizationId, {
        email: form.email.trim(),
        display_name: form.displayName.trim(),
        ...(form.password ? { password: form.password } : {}),
        permissions: form.permissions
      });
      setDialogOpen(false);
      setEditingMember(null);
      await refreshMembers();
      onNotice(
        result?.message ??
          (editingMember
            ? 'Member permissions updated.'
            : 'Member added to the organization.'),
        result ? 'info' : 'success'
      );
    } catch (saveError) {
      onNotice(errorMessage(saveError), 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveMember = async () => {
    if (!removeTarget) return;
    setSaving(true);
    try {
      await removeOrganizationMember(organizationId, removeTarget.user_id);
      setRemoveTarget(null);
      await refreshMembers();
      onNotice('Member removed from the organization.', 'success');
    } catch (removeError) {
      onNotice(errorMessage(removeError), 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Card className="content-card members-page-card">
        <CardContent>
          <Box className="section-heading members-heading">
            <Box>
              <Box className="members-title">
                <GroupsRoundedIcon color="primary" />
                <Typography variant="h6" sx={{ fontWeight: 750 }}>
                  User management
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary">
                {members.length} {members.length === 1 ? 'member' : 'members'}{' '}
                in this organization
              </Typography>
            </Box>
            <Box className="members-actions">
              <TextField
                className="search-field"
                placeholder="Search members"
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
              <Tooltip title="Refresh members">
                <span>
                  <IconButton
                    aria-label="Refresh members"
                    onClick={() => void refreshMembers()}
                    disabled={loading}
                  >
                    <RefreshRoundedIcon />
                  </IconButton>
                </span>
              </Tooltip>
              <Button
                variant="contained"
                startIcon={<AddRoundedIcon />}
                onClick={openAddDialog}
              >
                Add member
              </Button>
            </Box>
          </Box>

          {error && (
            <Alert
              severity="error"
              action={
                <Button
                  color="inherit"
                  size="small"
                  onClick={() => void refreshMembers()}
                >
                  Retry
                </Button>
              }
              sx={{ mb: 2 }}
            >
              {error}
            </Alert>
          )}

          {loading ? (
            <Box className="members-loader">
              <CircularProgress size={28} />
            </Box>
          ) : !members.length ? (
            <Box className="members-empty">
              <GroupsRoundedIcon />
              <Typography sx={{ fontWeight: 700 }}>No members found</Typography>
              <Typography variant="body2" color="text.secondary">
                Add someone to collaborate in this organization.
              </Typography>
            </Box>
          ) : (
            <TableContainer
              component={Paper}
              elevation={0}
              className="members-table-container"
            >
              <Table
                className="members-table"
                aria-label="Organization members"
              >
                <TableHead>
                  <TableRow>
                    <TableCell>MEMBER</TableCell>
                    <TableCell>ACCESS</TableCell>
                    <TableCell>JOINED</TableCell>
                    <TableCell align="right">ACTIONS</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {filteredMembers.map((member) => (
                    <TableRow key={member.user_id}>
                      <TableCell>
                        <Box className="member-identity">
                          <Typography variant="body2" sx={{ fontWeight: 700 }}>
                            {member.display_name || member.email}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {member.email}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box className="member-permissions">
                          {member.is_owner ? (
                            <span className="member-role-pill">
                              Owner · full access
                            </span>
                          ) : member.permissions.length ? (
                            member.permissions.map((permission) => (
                              <span
                                className="member-permission-pill"
                                key={permission}
                              >
                                {permissionDescriptions[permission].title}
                              </span>
                            ))
                          ) : (
                            <Typography
                              variant="caption"
                              color="text.secondary"
                            >
                              No permissions assigned
                            </Typography>
                          )}
                        </Box>
                      </TableCell>
                      <TableCell className="date-cell">
                        {formatJoinedDate(member.joined_at)}
                      </TableCell>
                      <TableCell align="right">
                        <Box className="member-row-actions">
                          {!member.is_owner && (
                            <>
                              <Tooltip
                                title={
                                  member.user_id === currentUserId
                                    ? 'Only the organization owner can change your permissions'
                                    : 'Assign permissions'
                                }
                              >
                                <span>
                                  <IconButton
                                    aria-label={`Assign permissions to ${member.email}`}
                                    size="small"
                                    disabled={member.user_id === currentUserId}
                                    onClick={() => openEditDialog(member)}
                                  >
                                    <EditRoundedIcon fontSize="small" />
                                  </IconButton>
                                </span>
                              </Tooltip>
                              <Tooltip
                                title={
                                  member.user_id === currentUserId
                                    ? 'You cannot remove your own membership here'
                                    : 'Remove member'
                                }
                              >
                                <span>
                                  <IconButton
                                    aria-label={`Remove ${member.email}`}
                                    size="small"
                                    color="error"
                                    disabled={member.user_id === currentUserId}
                                    onClick={() => setRemoveTarget(member)}
                                  >
                                    <DeleteOutlineRoundedIcon fontSize="small" />
                                  </IconButton>
                                </span>
                              </Tooltip>
                            </>
                          )}
                        </Box>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {filteredMembers.length === 0 && (
                <Typography
                  className="members-no-results"
                  variant="body2"
                  color="text.secondary"
                >
                  No members match “{search}”.
                </Typography>
              )}
            </TableContainer>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={dialogOpen}
        onClose={closeMemberDialog}
        fullWidth
        maxWidth="sm"
      >
        <Box
          component="form"
          onSubmit={(event) => void handleSaveMember(event)}
        >
          <DialogTitle sx={{ fontWeight: 750 }}>
            {editingMember ? 'Manage member access' : 'Add organization member'}
          </DialogTitle>
          <DialogContent className="dialog-fields">
            <Typography variant="body2" color="text.secondary">
              Assign only the permissions this member needs. The owner always
              has full access.
            </Typography>
            <TextField
              label="Email address"
              type="email"
              value={form.email}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  email: event.target.value
                }))
              }
              autoFocus
              required
              fullWidth
              disabled={Boolean(editingMember)}
            />
            <TextField
              label="Display name (optional)"
              value={form.displayName}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  displayName: event.target.value
                }))
              }
              helperText={
                editingMember
                  ? 'Display names are managed by the member.'
                  : undefined
              }
              fullWidth
              disabled={Boolean(editingMember)}
            />
            {!editingMember && (
              <TextField
                label="Temporary password (new accounts only)"
                type="password"
                value={form.password}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    password: event.target.value
                  }))
                }
                helperText="Leave blank for an existing account. New accounts require 12–72 characters."
                slotProps={{ htmlInput: { minLength: 12, maxLength: 72 } }}
                fullWidth
              />
            )}
            <Box>
              <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>
                Permissions
              </Typography>
              <FormGroup>
                {organizationPermissions.map((permission) => (
                  <FormControlLabel
                    key={permission}
                    control={
                      <Checkbox
                        checked={form.permissions.includes(permission)}
                        onChange={() => togglePermission(permission)}
                      />
                    }
                    label={
                      <Box>
                        <Typography variant="body2" sx={{ fontWeight: 650 }}>
                          {permissionDescriptions[permission].title}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {permissionDescriptions[permission].description}
                        </Typography>
                      </Box>
                    }
                  />
                ))}
              </FormGroup>
            </Box>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2.5 }}>
            <Button onClick={closeMemberDialog} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" variant="contained" disabled={saving}>
              {saving ? (
                <CircularProgress size={20} color="inherit" />
              ) : editingMember ? (
                'Save permissions'
              ) : (
                'Add member'
              )}
            </Button>
          </DialogActions>
        </Box>
      </Dialog>

      <Dialog
        open={Boolean(removeTarget)}
        onClose={() => !saving && setRemoveTarget(null)}
      >
        <DialogTitle sx={{ fontWeight: 750 }}>Remove this member?</DialogTitle>
        <DialogContent>
          <Typography color="text.secondary">
            {removeTarget?.email} will lose access to this organization. Their
            account will not be deleted.
          </Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2.5 }}>
          <Button onClick={() => setRemoveTarget(null)} disabled={saving}>
            Cancel
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => void handleRemoveMember()}
            disabled={saving}
          >
            {saving ? (
              <CircularProgress size={20} color="inherit" />
            ) : (
              'Remove member'
            )}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
