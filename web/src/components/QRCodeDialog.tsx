import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography
} from '@mui/material';
import FileDownloadRoundedIcon from '@mui/icons-material/FileDownloadRounded';
import type { ShortLink } from '../api/api';
import { getQRCode } from '../api/api';
import { errorMessage } from '../utils/errors';

export default function QRCodeDialog({
  link,
  onClose,
  onError
}: {
  link: ShortLink;
  onClose: () => void;
  onError: (message: string) => void;
}) {
  const [imageUrl, setImageUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    let objectUrl = '';

    getQRCode(link.short_url_code)
      .then((image) => {
        if (!active) return;
        objectUrl = URL.createObjectURL(image);
        setImageUrl(objectUrl);
      })
      .catch((cause: unknown) => {
        if (!active) return;
        const message = errorMessage(cause);
        setError(message);
        onError(message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [link.short_url_code, onError]);

  const handleDownload = () => {
    if (!imageUrl) return;
    const download = document.createElement('a');
    download.href = imageUrl;
    download.download = `${link.short_url_code}.png`;
    document.body.append(download);
    download.click();
    download.remove();
  };

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ fontWeight: 750 }}>QR code</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          Scan this code to open your short link.
        </Typography>
        <Box
          sx={{
            px: 1.5,
            py: 1.25,
            mb: 2,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: 'background.default',
            overflowWrap: 'anywhere',
            color: 'text.primary',
            fontSize: 14
          }}
        >
          {link.short_url}
        </Box>
        <Box
          sx={{
            minHeight: 240,
            display: 'grid',
            placeItems: 'center',
            borderRadius: 2,
            bgcolor: 'background.default'
          }}
        >
          {loading ? (
            <CircularProgress />
          ) : imageUrl ? (
            <Box
              component="img"
              src={imageUrl}
              alt={`QR code for ${link.short_url}`}
              sx={{ display: 'block', width: 220, maxWidth: '100%' }}
            />
          ) : error ? (
            <Typography color="error" variant="body2">
              {error}
            </Typography>
          ) : (
            <CircularProgress />
          )}
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.5 }}>
        <Button onClick={onClose}>Close</Button>
        <Button
          variant="contained"
          startIcon={<FileDownloadRoundedIcon />}
          disabled={!imageUrl}
          onClick={handleDownload}
        >
          Download QR code
        </Button>
      </DialogActions>
    </Dialog>
  );
}
