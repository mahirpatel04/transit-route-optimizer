export function formatClockTime(isoString) {
  const d = new Date(isoString);
  if (Number.isNaN(d.getTime())) return isoString;
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

export function formatDuration(totalSeconds) {
  const minutes = Math.round(totalSeconds / 60);
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours} hr` : `${hours} hr ${remainder} min`;
}

export function formatRelativeTime(isoString, now = new Date()) {
  const then = new Date(isoString);
  if (Number.isNaN(then.getTime())) return isoString;

  const seconds = Math.round((now.getTime() - then.getTime()) / 1000);

  if (seconds < 60) return 'just now';

  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;

  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`;

  const days = Math.round(hours / 24);
  if (days < 7) return `${days} day${days === 1 ? '' : 's'} ago`;

  return then.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}
