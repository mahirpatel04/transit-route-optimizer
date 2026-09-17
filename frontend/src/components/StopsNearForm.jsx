import { useState } from 'react';
import { API_URL } from '../config';
import { isValidLatLon } from '../validation';
import { formatDistance } from '../format';

export default function StopsNearForm() {
  const [lat, setLat] = useState('');
  const [lon, setLon] = useState('');
  const [status, setStatus] = useState('idle'); // idle | loading | error | success
  const [error, setError] = useState('');
  const [stops, setStops] = useState([]);
  const [locating, setLocating] = useState(false);

  function handleUseMyLocation() {
    if (!navigator.geolocation) {
      setStatus('error');
      setError('Location is not supported in this browser — enter coordinates manually.');
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setLat(String(position.coords.latitude));
        setLon(String(position.coords.longitude));
        setLocating(false);
      },
      () => {
        setLocating(false);
        setStatus('error');
        setError('Could not get your location — enter coordinates manually.');
      }
    );
  }

  async function handleSubmit(e) {
    e.preventDefault();
    const latNum = parseFloat(lat);
    const lonNum = parseFloat(lon);

    if (!isValidLatLon(latNum, lonNum)) {
      setStatus('error');
      setError('Enter a valid latitude (-90 to 90) and longitude (-180 to 180).');
      return;
    }

    setStatus('loading');
    try {
      const res = await fetch(`${API_URL}/stops/near?lat=${latNum}&lon=${lonNum}`);
      const data = await res.json();
      if (!res.ok) {
        setStatus('error');
        setError(data.error || 'Request failed.');
        return;
      }
      setStops(Array.isArray(data) ? data : []);
      setStatus('success');
    } catch {
      setStatus('error');
      setError('Network error, try again.');
    }
  }

  return (
    <section className="panel">
      <h2>Nearby stops</h2>
      <form onSubmit={handleSubmit}>
        <div className="field-row">
          <label>
            Latitude
            <input
              type="number"
              step="any"
              value={lat}
              onChange={(e) => setLat(e.target.value)}
            />
          </label>
          <label>
            Longitude
            <input
              type="number"
              step="any"
              value={lon}
              onChange={(e) => setLon(e.target.value)}
            />
          </label>
        </div>
        <div className="button-row">
          <button type="button" onClick={handleUseMyLocation} disabled={locating}>
            {locating ? 'Locating…' : 'Use my location'}
          </button>
          <button type="submit" disabled={status === 'loading'}>
            {status === 'loading' ? 'Searching…' : 'Find nearby stops'}
          </button>
        </div>
      </form>

      {status === 'loading' && (
        <ul className="skeleton-list" aria-hidden="true">
          <li />
          <li />
          <li />
        </ul>
      )}

      {status === 'error' && <p role="alert" className="inline-error">{error}</p>}

      {status === 'success' && (
        stops.length === 0 ? (
          <p className="empty-state">No stops found nearby.</p>
        ) : (
          <ul className="stop-list">
            {stops.map((s) => (
              <li key={s.stop_id}>
                <span className="stop-name">{s.stop_name}</span>
                <span className="stop-distance">{formatDistance(s.distance_meters)}</span>
              </li>
            ))}
          </ul>
        )
      )}
    </section>
  );
}
