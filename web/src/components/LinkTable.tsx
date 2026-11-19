import { Box, IconButton, Typography } from "@mui/material";
import ArrowOutwardRoundedIcon from "@mui/icons-material/ArrowOutwardRounded";
import ContentCopyRoundedIcon from "@mui/icons-material/ContentCopyRounded";
import DeleteOutlineRoundedIcon from "@mui/icons-material/DeleteOutlineRounded";
import LinkRoundedIcon from "@mui/icons-material/LinkRounded";
import QrCode2RoundedIcon from "@mui/icons-material/QrCode2Rounded";
import TuneRoundedIcon from "@mui/icons-material/TuneRounded";
import type { ShortLink } from "../api/api";

const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
const localDateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: localTimeZone,
});

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(new Date(value));
}

function formatExpiry(value: string | null) {
  return value ? localDateTimeFormatter.format(new Date(value)) : "No expiry";
}

const countryNames = new Intl.DisplayNames(undefined, { type: "region" });

export default function LinkTable({
  canManageLinks,
  links,
  currentTime,
  onCopy,
  onQRCode,
  onToggle,
  onRevoke,
}: {
  canManageLinks: boolean;
  links: ShortLink[];
  currentTime: number;
  onCopy: (url: string) => void;
  onQRCode: (link: ShortLink) => void;
  onToggle: (link: ShortLink) => void;
  onRevoke: (link: ShortLink) => void;
}) {
  if (!links.length) {
    return (
      <Box className="empty-links">
        <span className="empty-icon">
          <LinkRoundedIcon className="link-icon" />
        </span>
        <Typography sx={{ fontWeight: 700 }}>No links yet</Typography>
        <Typography variant="body2" color="text.secondary">
          {canManageLinks
            ? "Create your first short link to see it here."
            : "There are no links in this workspace yet."}
        </Typography>
      </Box>
    );
  }

  return (
    <Box className="table-scroll">
      <table className="links-table">
        <thead>
          <tr>
            <th>LINK</th>
            <th>CLICKS</th>
            <th>COUNTRIES</th>
            <th>STATUS</th>
            <th>CREATED</th>
            <th>EXPIRES</th>
            <th aria-label="Actions" />
          </tr>
        </thead>
        <tbody>
          {links.map((link) => {
            const expired = Boolean(
              link.expires_at &&
              new Date(link.expires_at).getTime() <= currentTime,
            );
            const displayedStatus =
              link.status === "active" && expired ? "expired" : link.status;
            return (
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
                      <ArrowOutwardRoundedIcon fontSize="inherit" />
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
                <td className="country-cell">
                  {link.country_codes.length
                    ? link.country_codes
                        .map((code) => countryNames.of(code) ?? code)
                        .join(", ")
                    : "—"}
                </td>
                <td>
                  <span className={`status-pill status-${displayedStatus}`}>
                    {displayedStatus}
                  </span>
                </td>
                <td className="date-cell">{formatDate(link.created_at)}</td>
                <td className="date-cell">{formatExpiry(link.expires_at)}</td>
                <td>
                  <Box className="row-actions">
                    <IconButton
                      title="Copy short link"
                      size="small"
                      onClick={() => onCopy(link.short_url)}
                    >
                      <ContentCopyRoundedIcon fontSize="small" />
                    </IconButton>
                    {link.status === "active" &&
                      (!link.expires_at ||
                        new Date(link.expires_at).getTime() > currentTime) && (
                        <IconButton
                          title="Download QR code"
                          size="small"
                          onClick={() => onQRCode(link)}
                        >
                          <QrCode2RoundedIcon fontSize="small" />
                        </IconButton>
                      )}
                    {canManageLinks &&
                      link.status !== "revoked" &&
                      !expired && (
                        <IconButton
                          title={
                            link.status === "active"
                              ? "Disable link"
                              : "Enable link"
                          }
                          size="small"
                          onClick={() => onToggle(link)}
                        >
                          <TuneRoundedIcon fontSize="small" />
                        </IconButton>
                      )}
                    {canManageLinks && link.status !== "revoked" && (
                      <>
                        <IconButton
                          title="Revoke link"
                          size="small"
                          color="error"
                          onClick={() => onRevoke(link)}
                        >
                          <DeleteOutlineRoundedIcon fontSize="small" />
                        </IconButton>
                      </>
                    )}
                  </Box>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </Box>
  );
}
