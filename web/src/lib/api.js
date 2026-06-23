// Thin wrappers around the Go JSON API.

async function jsonOrThrow(res) {
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body && body.error) msg = body.error;
    } catch (_) {}
    throw new Error(msg);
  }
  return res.json();
}

export function getState() {
  return fetch('/api/state').then(jsonOrThrow);
}

export function getStats() {
  return fetch('/api/stats').then(jsonOrThrow);
}

export function uploadBackup(file) {
  const fd = new FormData();
  fd.append('file', file);
  return fetch('/api/backup', { method: 'POST', body: fd }).then(jsonOrThrow);
}

export function generatePlaylist(request) {
  return fetch('/api/playlist', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  }).then(jsonOrThrow);
}

// Triggers a browser download of the .m3u for the given request.
export async function downloadM3U(request, name) {
  const res = await fetch('/api/playlist.m3u', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...request, name }),
  });
  if (!res.ok) throw new Error(`download failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${(name || 'playlist').replace(/[\\/:*?"<>|]+/g, '-')}.m3u`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
