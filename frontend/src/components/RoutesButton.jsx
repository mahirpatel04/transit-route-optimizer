import { useMemo, useState } from 'react';
import { API_URL } from '../config';
import RouteBullet from './RouteBullet';

export default function RoutesButton() {
  const [status, setStatus] = useState('idle'); // idle | loading | error | success
  const [error, setError] = useState('');
  const [routes, setRoutes] = useState([]);
  const [filter, setFilter] = useState('');

  async function handleClick() {
    setStatus('loading');
    try {
      const res = await fetch(`${API_URL}/routes`);
      const data = await res.json();
      if (!res.ok) {
        setStatus('error');
        setError(data.error || 'Request failed.');
        return;
      }
      setRoutes(Array.isArray(data) ? data : []);
      setStatus('success');
    } catch {
      setStatus('error');
      setError('Network error, try again.');
    }
  }

  const visibleRoutes = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return routes;
    return routes.filter(
      (r) => r.route_id.toLowerCase().includes(q) || r.route_name.toLowerCase().includes(q)
    );
  }, [routes, filter]);

  return (
    <section className="panel">
      <div className="panel-heading">
        <h2>Routes</h2>
        <button onClick={handleClick} disabled={status === 'loading'}>
          {status === 'loading' ? 'Loading…' : 'Get routes'}
        </button>
      </div>

      {status === 'loading' && (
        <ul className="skeleton-list" aria-hidden="true">
          <li />
          <li />
          <li />
        </ul>
      )}

      {status === 'error' && <p role="alert" className="inline-error">{error}</p>}

      {status === 'success' && (
        <>
          <input
            type="search"
            className="filter-input"
            placeholder="Filter routes…"
            aria-label="Filter routes"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          {visibleRoutes.length === 0 ? (
            <p className="empty-state">No routes match "{filter}".</p>
          ) : (
            <ul className="route-list">
              {visibleRoutes.map((r) => (
                <li key={r.route_id}>
                  <RouteBullet routeId={r.route_id} />
                  <span>{r.route_name}</span>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}
