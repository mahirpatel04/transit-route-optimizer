import { useEffect, useState } from 'react';
import { API_URL } from '../config';
import { formatClockTime, formatDuration } from '../format';
import { collapseAdjacentWalks } from '../legs';
import { POPULAR_PLACES } from '../places';
import RouteBullet from './RouteBullet';

// The database scales down to zero when idle, so the first request after a
// quiet period can take several seconds to wake it back up — this threshold
// is how long we wait before explaining that to the user, rather than
// leaving them staring at a bare "Finding route…".
const SLOW_REQUEST_MESSAGE_DELAY_MS = 2000;

export default function RouteFinder() {
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [optimize, setOptimize] = useState(false);
  const [status, setStatus] = useState('idle'); // idle | loading | error | success
  const [error, setError] = useState('');
  const [route, setRoute] = useState(null);
  const [isSlow, setIsSlow] = useState(false);

  useEffect(() => {
    if (status !== 'loading') {
      setIsSlow(false);
      return;
    }
    const timer = setTimeout(() => setIsSlow(true), SLOW_REQUEST_MESSAGE_DELAY_MS);
    return () => clearTimeout(timer);
  }, [status]);

  async function handleSubmit(e) {
    e.preventDefault();
    if (!from.trim() || !to.trim()) {
      setStatus('error');
      setError('Enter both a starting point and a destination.');
      return;
    }

    setStatus('loading');
    try {
      const params = new URLSearchParams({ from, to });
      if (optimize) params.set('optimize', 'true');
      const res = await fetch(`${API_URL}/route?${params}`);
      const data = await res.json();
      if (!res.ok) {
        setStatus('error');
        setError(data.error || 'Request failed.');
        return;
      }
      setRoute(data);
      setStatus('success');
    } catch {
      setStatus('error');
      setError('Network error, try again.');
    }
  }

  return (
    <section className="panel">
      <h2>Find a route</h2>
      <form onSubmit={handleSubmit}>
        <div className="field-row">
          <label>
            From
            <input
              type="text"
              placeholder="Times Square, NYC"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
            />
            <SuggestionChips label="Suggested starting points" onSelect={setFrom} />
          </label>
          <label>
            To
            <input
              type="text"
              placeholder="Grand Central Terminal, NYC"
              value={to}
              onChange={(e) => setTo(e.target.value)}
            />
            <SuggestionChips label="Suggested destinations" onSelect={setTo} />
          </label>
        </div>
        <div className="optimize-toggle-row">
          <span className={`toggle-label${!optimize ? ' toggle-label-active' : ''}`}>
            Optimize to get to closest station
          </span>
          <label className="toggle-switch">
            <input
              type="checkbox"
              role="switch"
              checked={optimize}
              onChange={(e) => setOptimize(e.target.checked)}
              aria-label="Optimize for fastest trip instead of closest station"
            />
            <span className="toggle-track">
              <span className="toggle-thumb" />
            </span>
          </label>
          <span className={`toggle-label${optimize ? ' toggle-label-active' : ''}`}>
            Optimize for fastest trip (may mean more walking)
          </span>
        </div>
        <div className="button-row">
          <button type="submit" disabled={status === 'loading'}>
            {status === 'loading' ? 'Finding route…' : 'Find route'}
          </button>
        </div>
      </form>

      {status === 'loading' && (
        <>
          {isSlow && <p className="loading-note">Spinning up the database…</p>}
          <ul className="skeleton-list" aria-hidden="true">
            <li />
            <li />
            <li />
          </ul>
        </>
      )}

      {status === 'error' && <p role="alert" className="inline-error">{error}</p>}

      {status === 'success' && (
        <>
          <p className="route-summary">Total time: {formatDuration(route.total_time_seconds)}</p>
          <ol className="leg-list">
            {collapseAdjacentWalks(route.legs).map((leg, i) => (
              <li key={i} className={`leg leg-${leg.kind}`}>
                {leg.kind === 'ride' ? (
                  <RouteBullet routeId={leg.route_id} />
                ) : (
                  <span className="leg-walk-icon" aria-hidden="true">
                    🚶
                  </span>
                )}
                <span className="leg-detail">
                  {leg.kind === 'ride'
                    ? `Take the ${leg.route_id} train from ${leg.from_stop_name} to ${leg.to_stop_name}`
                    : `Walk to ${leg.to_stop_name}`}
                </span>
                <span className="leg-times">
                  {formatClockTime(leg.depart_at)} – {formatClockTime(leg.arrive_at)}
                </span>
              </li>
            ))}
          </ol>
        </>
      )}
    </section>
  );
}

function SuggestionChips({ label, onSelect }) {
  return (
    <div className="suggestion-chips" role="group" aria-label={label}>
      {POPULAR_PLACES.map((place) => (
        <button type="button" key={place} className="suggestion-chip" onClick={() => onSelect(place)}>
          {place.replace(', NYC', '')}
        </button>
      ))}
    </div>
  );
}
