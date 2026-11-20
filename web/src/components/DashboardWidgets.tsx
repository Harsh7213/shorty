import type { ReactNode } from 'react';
import {
  Box,
  Card,
  CardContent,
  CircularProgress,
  ToggleButton,
  ToggleButtonGroup,
  Typography
} from '@mui/material';
import type { OrganizationAnalytics, ShortLink } from '../api/api';

export function StatCard({
  label,
  value,
  icon,
  tone,
  footnote
}: {
  label: string;
  value: string;
  icon: ReactNode;
  tone: string;
  footnote: string;
}) {
  return (
    <Card className="stat-card">
      <CardContent>
        <Box className={`stat-icon stat-${tone}`}>{icon}</Box>
        <Typography
          variant="body2"
          color="text.secondary"
          className="stat-label"
        >
          {label}
        </Typography>
        <Typography variant="h4" sx={{ fontWeight: 750 }}>
          {value}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {footnote}
        </Typography>
      </CardContent>
    </Card>
  );
}

export function CenteredLoader() {
  return (
    <Box className="centered-loader">
      <CircularProgress size={26} />
    </Box>
  );
}

function LinkSparkline({
  clicks,
  shortUrl,
  period
}: {
  clicks: OrganizationAnalytics['links'][number]['clicks'];
  shortUrl: string;
  period: OrganizationAnalytics['period'];
}) {
  const maxClicks = Math.max(1, ...clicks.map((day) => day.clicks));
  const points = clicks.map((day, index) => ({
    x: clicks.length > 1 ? 3 + (index * 174) / (clicks.length - 1) : 90,
    y: 33 - (day.clicks / maxClicks) * 27,
    ...day
  }));

  return (
    <svg
      className="link-sparkline"
      viewBox="0 0 180 42"
      role="img"
      aria-label={`Daily visits for ${shortUrl} over the last ${period}`}
    >
      <line x1="2" y1="34" x2="178" y2="34" className="chart-axis" />
      {points.length > 1 && (
        <polyline
          points={points.map(({ x, y }) => `${x},${y}`).join(' ')}
          className="chart-line"
        />
      )}
      {points.map((point) => (
        <circle
          key={point.date}
          cx={point.x}
          cy={point.y}
          r="1.8"
          className="chart-point"
        >
          <title>{`${point.date}: ${point.clicks} visits`}</title>
        </circle>
      ))}
    </svg>
  );
}

function CountryClicksChart({
  countries
}: {
  countries: OrganizationAnalytics['countries'];
}) {
  const maxClicks = Math.max(1, ...countries.map((country) => country.clicks));
  const countryNames = new Intl.DisplayNames(undefined, { type: 'region' });

  return (
    <Box className="country-clicks-chart">
      <Box className="section-heading">
        <Box>
          <Typography variant="h6" sx={{ fontWeight: 750 }}>
            Clicks by country
          </Typography>
          <Typography variant="body2" color="text.secondary">
            All links · all-time clicks
          </Typography>
        </Box>
      </Box>
      {countries.length ? (
        <Box
          className="country-clicks-list"
          role="img"
          aria-label="All-time link clicks by country"
        >
          {countries.map((country) => {
            const name = country.country_code
              ? (countryNames.of(country.country_code) ?? country.country_code)
              : 'Unknown';
            return (
              <Box
                className="country-click-row"
                key={country.country_code || 'unknown'}
              >
                <Box className="country-click-heading">
                  <Typography variant="body2">{name}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {country.clicks.toLocaleString()}
                  </Typography>
                </Box>
                <Box className="country-click-track">
                  <Box
                    className="country-click-bar"
                    sx={{ width: `${(country.clicks / maxClicks) * 100}%` }}
                  />
                </Box>
              </Box>
            );
          })}
        </Box>
      ) : (
        <Typography
          className="analytics-message"
          color="text.secondary"
          variant="body2"
        >
          No click data is available yet.
        </Typography>
      )}
    </Box>
  );
}

export function LinkAnalyticsTable({
  links,
  analytics,
  period,
  loading,
  error,
  onPeriodChange
}: {
  links: ShortLink[];
  analytics: OrganizationAnalytics | null;
  period: OrganizationAnalytics['period'];
  loading: boolean;
  error: string;
  onPeriodChange: (period: OrganizationAnalytics['period']) => void;
}) {
  const dailyClicks = new Map(
    analytics?.links.map((link) => [link.link_id, link.clicks]) ?? []
  );

  return (
    <Card className="content-card link-analytics-card">
      <CardContent>
        <Box className="section-heading">
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 750 }}>
              Link performance
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Daily visits for each link
            </Typography>
          </Box>
          <ToggleButtonGroup
            size="small"
            exclusive
            value={period}
            onChange={(_, value: OrganizationAnalytics['period'] | null) =>
              value && onPeriodChange(value)
            }
            aria-label="Analytics time period"
          >
            <ToggleButton value="week">Week</ToggleButton>
            <ToggleButton value="month">Month</ToggleButton>
          </ToggleButtonGroup>
        </Box>
        {loading ? (
          <CenteredLoader />
        ) : error ? (
          <Typography
            className="analytics-message"
            color="error"
            variant="body2"
          >
            {error}
          </Typography>
        ) : (
          <>
            <CountryClicksChart countries={analytics?.countries ?? []} />
            {links.length ? (
              <Box className="table-scroll">
                <table className="links-table link-analytics-table">
                  <thead>
                    <tr>
                      <th>LINK</th>
                      <th>ALL-TIME CLICKS</th>
                      <th>DAILY VISITS ({period.toUpperCase()})</th>
                    </tr>
                  </thead>
                  <tbody>
                    {links.map((link) => (
                      <tr key={link.id}>
                        <td>
                          <Box className="link-cell">
                            <a
                              className="short-link"
                              href={link.short_url}
                              target="_blank"
                              rel="noreferrer"
                            >
                              {link.short_url}
                            </a>
                            <span className="destination" title={link.long_url}>
                              {link.long_url}
                            </span>
                          </Box>
                        </td>
                        <td>
                          <span className="click-count">
                            {link.total_clicks.toLocaleString()}
                          </span>
                        </td>
                        <td>
                          <LinkSparkline
                            clicks={dailyClicks.get(link.id) ?? []}
                            shortUrl={link.short_url}
                            period={period}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Box>
            ) : (
              <Typography
                className="analytics-message"
                color="text.secondary"
                variant="body2"
              >
                Create a link to see its access graph here.
              </Typography>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
