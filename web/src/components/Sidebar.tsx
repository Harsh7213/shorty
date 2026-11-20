import { Avatar, Box, Typography } from '@mui/material';
import BarChartRoundedIcon from '@mui/icons-material/BarChartRounded';
import DashboardRoundedIcon from '@mui/icons-material/DashboardRounded';
import GroupsRoundedIcon from '@mui/icons-material/GroupsRounded';
import LinkRoundedIcon from '@mui/icons-material/LinkRounded';
import LogoutRoundedIcon from '@mui/icons-material/LogoutRounded';
import type { User } from '../api/api';

export type DashboardView = 'overview' | 'links' | 'analytics' | 'members';

export default function Sidebar({
  user,
  view,
  linkCount,
  canManageUsers,
  onViewChange,
  onSignOut
}: {
  user: User;
  view: DashboardView;
  linkCount: number;
  canManageUsers: boolean;
  onViewChange: (view: DashboardView) => void;
  onSignOut: () => void;
}) {
  return (
    <aside className="sidebar">
      <Box className="sidebar-brand">
        <span className="brand-mark">
          <img src="/favicon.svg" width={38} height={38} alt="" />
        </span>
        <Typography variant="h5" sx={{ fontWeight: 800 }}>
          shorty
        </Typography>
      </Box>
      <Typography className="nav-caption">WORKSPACE</Typography>
      <button
        className={`nav-item ${view === 'overview' ? 'nav-item-active' : ''}`}
        onClick={() => onViewChange('overview')}
      >
        <DashboardRoundedIcon fontSize="small" /> Overview
      </button>
      <button
        className={`nav-item ${view === 'links' ? 'nav-item-active' : ''}`}
        onClick={() => onViewChange('links')}
      >
        <LinkRoundedIcon className="link-icon" fontSize="small" /> My links
        <span className="nav-count">{linkCount}</span>
      </button>
      <button
        className={`nav-item ${view === 'analytics' ? 'nav-item-active' : ''}`}
        onClick={() => onViewChange('analytics')}
      >
        <BarChartRoundedIcon fontSize="small" /> Link analytics
      </button>
      {canManageUsers && (
        <button
          className={`nav-item ${view === 'members' ? 'nav-item-active' : ''}`}
          onClick={() => onViewChange('members')}
        >
          <GroupsRoundedIcon fontSize="small" /> User management
        </button>
      )}
      <Box className="sidebar-bottom">
        <Box className="sidebar-tip">
          <span className="tip-icon">
            <BarChartRoundedIcon />
          </span>
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
            Make every link count
          </Typography>
          <Typography variant="caption">
            Track clicks and keep your links organized.
          </Typography>
        </Box>
        <button className="profile-button" onClick={onSignOut}>
          <Avatar className="profile-avatar">
            {(user.display_name || user.email).slice(0, 1).toUpperCase()}
          </Avatar>
          <span className="profile-text">
            <strong>{user.display_name || 'Shorty user'}</strong>
            <small>{user.email}</small>
          </span>
          <LogoutRoundedIcon fontSize="small" />
        </button>
      </Box>
    </aside>
  );
}
