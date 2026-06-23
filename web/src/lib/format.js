export function compact(n) {
  if (n == null) return '0';
  return new Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 1 }).format(n);
}

export function num(n) {
  return new Intl.NumberFormat('en').format(Math.round(n || 0));
}

export function pct(v) {
  return `${Math.round((v || 0) * 100)}%`;
}

export function hours(h) {
  if (!h) return '0h';
  if (h < 1) return `${Math.round(h * 60)}m`;
  return `${h.toFixed(h < 10 ? 1 : 0)}h`;
}

export function titleCase(s) {
  if (!s) return '';
  return s.charAt(0).toUpperCase() + s.slice(1);
}
