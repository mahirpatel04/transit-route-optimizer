import { useEffect, useState } from 'react';
import { API_URL } from '../config';
import { formatRelativeTime } from '../format';

export default function Header() {
  const [lastFetchTime, setLastFetchTime] = useState(null);

  useEffect(() => {
    async function fetchLastFetchTime() {
      try {
        const res = await fetch(`${API_URL}/ingest-time`);
        if (!res.ok) throw new Error('request failed');
        const data = await res.json();
        setLastFetchTime(data.last_fetch_time || null);
      } catch {
        setLastFetchTime(null);
      }
    }
    fetchLastFetchTime();
  }, []);

  return (
    <header className="app-header">
      <span className="freshness" title={lastFetchTime || undefined}>
        Last updated: {lastFetchTime ? formatRelativeTime(lastFetchTime) : '—'}
      </span>
    </header>
  );
}
