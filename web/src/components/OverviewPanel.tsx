import { Box, Button, Card, CardContent, Typography } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import ArrowOutwardRoundedIcon from '@mui/icons-material/ArrowOutwardRounded';
import BarChartRoundedIcon from '@mui/icons-material/BarChartRounded';
import LinkRoundedIcon from '@mui/icons-material/LinkRounded';
import type { ShortLink } from '../api/api';
import { CenteredLoader, StatCard } from './DashboardWidgets';
import LinkTable from './LinkTable';

export default function OverviewPanel({
  userName,
  canManageLinks,
  links,
  linksLoading,
  currentTime,
  onCreateLink,
  onViewAllLinks,
  onCopy,
  onQRCode,
  onToggle,
  onRevoke
}: {
  userName: string;
  canManageLinks: boolean;
  links: ShortLink[];
  linksLoading: boolean;
  currentTime: number;
  onCreateLink: () => void;
  onViewAllLinks: () => void;
  onCopy: (url: string) => void;
  onQRCode: (link: ShortLink) => void;
  onToggle: (link: ShortLink) => void;
  onRevoke: (link: ShortLink) => void;
}) {
  const activeLinks = links.filter(
    (link) =>
      link.status === 'active' &&
      (!link.expires_at || new Date(link.expires_at).getTime() > currentTime)
  );
  const totalClicks = links.reduce(
    (total, link) => total + link.total_clicks,
    0
  );
  const topLinks = [...links]
    .sort((a, b) => b.total_clicks - a.total_clicks)
    .slice(0, 4);
  const maxClicks = Math.max(1, ...topLinks.map((link) => link.total_clicks));

  return (
    <>
      <Box className="welcome-row">
        <Box>
          <Typography variant="h5" sx={{ fontWeight: 750 }}>
            Good to see you, {userName} 👋
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 0.5 }}>
            Here's what's happening with your short links.
          </Typography>
        </Box>
        {canManageLinks && (
          <Button
            variant="contained"
            startIcon={<AddRoundedIcon />}
            onClick={onCreateLink}
          >
            Shorten a URL
          </Button>
        )}
      </Box>
      <Box className="stats-grid">
        <StatCard
          label="Total links"
          value={links.length.toLocaleString()}
          icon={<LinkRoundedIcon className="link-icon" />}
          tone="purple"
          footnote="Across your workspace"
        />
        <StatCard
          label="Total clicks"
          value={totalClicks.toLocaleString()}
          icon={<BarChartRoundedIcon />}
          tone="mint"
          footnote="All-time link visits"
        />
        <StatCard
          label="Active links"
          value={activeLinks.length.toLocaleString()}
          icon={<ArrowOutwardRoundedIcon />}
          tone="orange"
          footnote="Ready to redirect"
        />
      </Box>
      <Box className="overview-grid">
        <Card className="content-card recent-card">
          <CardContent>
            <Box className="section-heading">
              <Box>
                <Typography variant="h6" sx={{ fontWeight: 750 }}>
                  Recent links
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Your latest shortened URLs
                </Typography>
              </Box>
              <Button size="small" onClick={onViewAllLinks}>
                View all
              </Button>
            </Box>
            {linksLoading ? (
              <CenteredLoader />
            ) : (
              <LinkTable
                canManageLinks={canManageLinks}
                showCountries={false}
                links={links.slice(0, 5)}
                currentTime={currentTime}
                onCopy={onCopy}
                onQRCode={onQRCode}
                onToggle={onToggle}
                onRevoke={onRevoke}
              />
            )}
          </CardContent>
        </Card>
        <Card className="content-card activity-card">
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 750 }}>
              Top links
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2.5 }}>
              Ranked by all-time clicks
            </Typography>
            {topLinks.length ? (
              topLinks.map((link) => (
                <Box className="top-link-row" key={link.id}>
                  <Box className="top-link-title">
                    <span className="top-link-dot" />
                    <Typography variant="body2" noWrap title={link.short_url}>
                      {link.short_url}
                    </Typography>
                    <strong>{link.total_clicks.toLocaleString()}</strong>
                  </Box>
                  <Box className="click-bar">
                    <span
                      style={{
                        width: `${(link.total_clicks / maxClicks) * 100}%`
                      }}
                    />
                  </Box>
                </Box>
              ))
            ) : (
              <Box className="top-empty">
                <BarChartRoundedIcon />
                <Typography variant="body2" color="text.secondary">
                  Click data will appear here as your links get visits.
                </Typography>
              </Box>
            )}
          </CardContent>
        </Card>
      </Box>
    </>
  );
}
