import { useEffect, useState } from 'react';
import { API_URL } from '../config';

export default function Header() {
  const [lastFetchTime, setLastFetchTime] = useState('—');

  useEffect(() => {
    fetch(`${API_URL}/ingest-time`)
      .then((res) => {
        if (!res.ok) throw new Error('request failed');
        return res.json();
      })
      .then((data) => setLastFetchTime(data.last_fetch_time || '—'))
      .catch(() => setLastFetchTime('—'));
  }, []);

  return (
    <header>
      <span>Last updated: {lastFetchTime}</span>
    </header>
  );
}
