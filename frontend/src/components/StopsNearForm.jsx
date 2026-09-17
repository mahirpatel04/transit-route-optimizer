import { useState } from 'react';
import { API_URL } from '../config';
import { isValidLatLon } from '../validation';

export default function StopsNearForm() {
  const [lat, setLat] = useState('');
  const [lon, setLon] = useState('');
  const [status, setStatus] = useState('idle'); // idle | loading | error | success
  const [error, setError] = useState('');
  const [stops, setStops] = useState([]);

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
      setStops(data);
      setStatus('success');
    } catch {
      setStatus('error');
      setError('Network error, try again.');
    }
  }

  return (
    <section>
      <form onSubmit={handleSubmit}>
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
        <button type="submit" disabled={status === 'loading'}>
          Find nearby stops
        </button>
      </form>
      {status === 'loading' && <p>Loading...</p>}
      {status === 'error' && <p role="alert">{error}</p>}
      {status === 'success' && (
        <ul>
          {stops.map((s) => (
            <li key={s.stop_id}>
              {s.stop_name} — {Math.round(s.distance_meters)}m
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
