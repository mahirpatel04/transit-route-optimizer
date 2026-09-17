import { useEffect, useState } from 'react';
import { API_URL } from '../config';

export default function Header() {
  const [lastFetchTime, setLastFetchTime] = useState('—');

  useEffect(() => {
    async function fetchLastFetchTime() {
      try {
        const res = await fetch(`${API_URL}/ingest-time`);
        if (!res.ok) throw new Error('request failed');
        const data = await res.json();
        setLastFetchTime(data.last_fetch_time || '—');
      } catch {
        setLastFetchTime('—');
      }
    }
    fetchLastFetchTime();
  }, []);

  return (
    <header>
      <span>Last updated: {lastFetchTime}</span>
    </header>
  );
}
