import type { ReactNode } from 'react';
import { useRef, useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  ToggleButton,
  ToggleButtonGroup,
  Typography
} from '@mui/material';
import { getLinkAnalytics } from '../api/api';
import type { OrganizationAnalytics, ShortLink } from '../api/api';
import { errorMessage } from '../utils/errors';

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
        <Typography variant="h6" color="text.secondary" className="stat-label">
          {label}
        </Typography>
        <Typography variant="h4" sx={{ fontWeight: 750 }}>
          {value}
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
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
  period,
  expanded = false
}: {
  clicks: OrganizationAnalytics['links'][number]['clicks'];
  shortUrl: string;
  period: OrganizationAnalytics['period'];
  expanded?: boolean;
}) {
  const maxClicks = Math.max(1, ...clicks.map((day) => day.clicks));
  const plot = { left: 56, right: 584, top: 26, bottom: 208 };
  const yTicks = [...new Set([0, Math.ceil(maxClicks / 2), maxClicks])];
  const points = clicks.map((day, index) => ({
    x:
      clicks.length > 1
        ? plot.left + (index * (plot.right - plot.left)) / (clicks.length - 1)
        : (plot.left + plot.right) / 2,
    y: plot.bottom - (day.clicks / maxClicks) * (plot.bottom - plot.top),
    ...day
  }));
  const dateIndexes = [
    ...new Set([0, Math.floor((clicks.length - 1) / 2), clicks.length - 1])
  ];

  return (
    <svg
      className="link-sparkline"
      style={expanded ? { width: '100%', height: 'auto' } : undefined}
      viewBox="0 0 600 260"
      role="img"
      aria-label={`Daily visits for ${shortUrl} over the last ${period}`}
    >
      {yTicks.map((tick) => {
        const y = plot.bottom - (tick / maxClicks) * (plot.bottom - plot.top);
        return (
          <g key={tick}>
            <line
              x1={plot.left}
              y1={y}
              x2={plot.right}
              y2={y}
              className="chart-axis"
            />
            <text
              x={plot.left - 10}
              y={y + 4}
              textAnchor="end"
              fill="#6b7280"
              fontSize="12"
            >
              {tick}
            </text>
          </g>
        );
      })}
      <line
        x1={plot.left}
        y1={plot.top}
        x2={plot.left}
        y2={plot.bottom}
        className="chart-axis"
      />
      <line
        x1={plot.left}
        y1={plot.bottom}
        x2={plot.right}
        y2={plot.bottom}
        className="chart-axis"
      />
      <text
        x="16"
        y="117"
        textAnchor="middle"
        transform="rotate(-90 16 117)"
        fill="#404551"
        fontSize="13"
      >
        Visits
      </text>
      <text
        x={(plot.left + plot.right) / 2}
        y="254"
        textAnchor="middle"
        fill="#404551"
        fontSize="13"
      >
        Date
      </text>
      {points.length > 1 && (
        <polyline
          points={points.map(({ x, y }) => `${x},${y}`).join(' ')}
          className="chart-line"
        />
      )}
      {dateIndexes.map((index) => {
        const point = points[index];
        if (!point) return null;
        return (
          <text
            key={point.date}
            x={point.x}
            y="230"
            textAnchor={
              index === 0
                ? 'start'
                : index === clicks.length - 1
                  ? 'end'
                  : 'middle'
            }
            fill="#6b7280"
            fontSize="12"
          >
            {point.date}
          </text>
        );
      })}
      {points.map((point) => (
        <circle
          key={point.date}
          cx={point.x}
          cy={point.y}
          r="4"
          className="chart-point"
        >
          <title>{`${point.date}: ${point.clicks} visits`}</title>
        </circle>
      ))}
    </svg>
  );
}

export function CountryClicksChart({
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
  organizationId,
  links,
  loading,
  error
}: {
  organizationId: number | '';
  links: ShortLink[];
  loading: boolean;
  error: string;
}) {
  const [period, setPeriod] = useState<OrganizationAnalytics['period']>('week');
  const [inspectedLink, setInspectedLink] = useState<ShortLink | null>(null);
  const [inspectedData, setInspectedData] = useState<{
    period: OrganizationAnalytics['period'];
    clicks: OrganizationAnalytics['links'][number]['clicks'];
  } | null>(null);
  const [inspectLoading, setInspectLoading] = useState(false);
  const [inspectError, setInspectError] = useState('');
  const inspectionRequestId = useRef(0);
  const inspectionCache = useRef(
    new Map<string, OrganizationAnalytics['links'][number]['clicks']>()
  );
  const inspectionRequests = useRef(
    new Map<string, Promise<OrganizationAnalytics['links'][number]['clicks']>>()
  );

  const loadInspectedLink = (
    link: ShortLink,
    selectedPeriod: OrganizationAnalytics['period']
  ) => {
    if (!organizationId) {
      setInspectError('Select a workspace before inspecting link analytics.');
      return;
    }
    const requestId = ++inspectionRequestId.current;
    const cacheKey = `${organizationId}:${link.id}:${selectedPeriod}`;
    const cachedClicks = inspectionCache.current.get(cacheKey);
    setInspectError('');
    if (cachedClicks !== undefined) {
      setInspectedData({ period: selectedPeriod, clicks: cachedClicks });
      setInspectLoading(false);
      return;
    }

    let request = inspectionRequests.current.get(cacheKey);
    if (!request) {
      const pendingRequest = getLinkAnalytics(
        organizationId,
        selectedPeriod,
        link.id
      ).then((result) => {
        const clicks =
          result.links.find((resultLink) => resultLink.link_id === link.id)
            ?.clicks ?? [];
        inspectionCache.current.set(cacheKey, clicks);
        return clicks;
      });
      inspectionRequests.current.set(cacheKey, pendingRequest);
      pendingRequest.then(
        () => {
          if (inspectionRequests.current.get(cacheKey) === pendingRequest) {
            inspectionRequests.current.delete(cacheKey);
          }
        },
        () => {
          if (inspectionRequests.current.get(cacheKey) === pendingRequest) {
            inspectionRequests.current.delete(cacheKey);
          }
        }
      );
      request = pendingRequest;
    }

    setInspectLoading(true);
    request
      .then((clicks) => {
        if (requestId === inspectionRequestId.current) {
          setInspectedData({ period: selectedPeriod, clicks });
        }
      })
      .catch((requestError: unknown) => {
        if (requestId === inspectionRequestId.current) {
          setInspectError(errorMessage(requestError));
        }
      })
      .finally(() => {
        if (requestId === inspectionRequestId.current) {
          setInspectLoading(false);
        }
      });
  };

  const closeInspection = () => {
    inspectionRequestId.current += 1;
    setInspectedLink(null);
    setInspectLoading(false);
  };

  return (
    <Card className="content-card link-analytics-card">
      <CardContent>
        <Box className="section-heading">
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 750 }}>
              Link performance
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Inspect a link to load its analytics
            </Typography>
          </Box>
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
            {links.length ? (
              <Box className="table-scroll">
                <table className="links-table link-analytics-table">
                  <thead>
                    <tr>
                      <th>LINK</th>
                      <th>ALL-TIME CLICKS</th>
                      <th>DAILY VISITS</th>
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
                          <Button
                            size="small"
                            onClick={() => {
                              setInspectedData(null);
                              setInspectedLink(link);
                              loadInspectedLink(link, period);
                            }}
                            aria-label={`Inspect daily visits for ${link.short_url}`}
                          >
                            Inspect
                          </Button>
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
      <Dialog
        open={inspectedLink !== null}
        onClose={closeInspection}
        fullWidth
        maxWidth="md"
        aria-labelledby="link-graph-title"
      >
        {inspectedLink && (
          <>
            <DialogTitle id="link-graph-title" sx={{ fontWeight: 750 }}>
              Link visits
            </DialogTitle>
            <DialogContent>
              <Box className="section-heading">
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    {inspectedLink.short_url}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Showing {inspectedData?.period ?? period}
                  </Typography>
                </Box>
                <ToggleButtonGroup
                  size="small"
                  exclusive
                  value={period}
                  onChange={(
                    _,
                    value: OrganizationAnalytics['period'] | null
                  ) => {
                    const selectedPeriod = value ?? period;
                    setPeriod(selectedPeriod);
                    loadInspectedLink(inspectedLink, selectedPeriod);
                  }}
                  aria-label="Chart time period"
                >
                  <ToggleButton value="week">Week</ToggleButton>
                  <ToggleButton value="month">Month</ToggleButton>
                </ToggleButtonGroup>
              </Box>
              {inspectLoading && !inspectedData && <CenteredLoader />}
              {inspectLoading && inspectedData && (
                <Box
                  sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}
                >
                  <CircularProgress size={16} />
                  <Typography variant="caption" color="text.secondary">
                    Refreshing {period} data…
                  </Typography>
                </Box>
              )}
              {inspectError && (
                <Typography color="error" variant="body2">
                  {inspectError}
                </Typography>
              )}
              {inspectedData?.clicks.length ? (
                <LinkSparkline
                  clicks={inspectedData.clicks}
                  shortUrl={inspectedLink.short_url}
                  period={inspectedData.period}
                  expanded
                />
              ) : !inspectLoading && !inspectError ? (
                <Typography color="text.secondary" variant="body2">
                  No daily visit data is available for this link yet.
                </Typography>
              ) : null}
            </DialogContent>
            <DialogActions sx={{ px: 3, pb: 2.5 }}>
              <Button onClick={closeInspection}>Close</Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </Card>
  );
}
