export function formatDistance(meters) {
  if (meters < 1000) return `${Math.round(meters)} m`;
  return `${(meters / 1000).toFixed(1)} km`;
}

// TODO(human): implement formatRelativeTime(isoString, now = new Date())
//
// Takes an ISO 8601 timestamp (e.g. "2026-09-17T18:00:28Z") and returns a
// short human-friendly string describing how long ago that was, e.g.
// "3 hours ago", "just now", "2 days ago". The `now` parameter defaults to
// the current time but can be overridden — that's what makes it testable
// without mocking the system clock.
//
// Design choices that are yours to make:
// - Where the bucket boundaries fall (seconds -> "just now"? minutes ->
//   hours -> days -> ... does it ever fall back to a plain date for very
//   old timestamps?)
// - Singular vs plural wording ("1 hour ago" vs "1 hours ago")
// - What happens on a malformed/unparseable isoString — this function is
//   called from Header, which already has its own em-dash fallback for
//   fetch failures, so returning something sane here (not throwing) keeps
//   that fallback meaningful rather than crashing the component
//
// Header.jsx calls this as formatRelativeTime(lastFetchTime) and renders
// the result directly next to "Last updated:".
export function formatRelativeTime(isoString, now = new Date()) {
}
