import { useState } from 'react';
import { API_URL } from '../config';

export default function RoutesButton() {
  const [status, setStatus] = useState('idle'); // idle | loading | error | success
  const [error, setError] = useState('');
  const [routes, setRoutes] = useState([]);

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

  return (
    <section>
      <button onClick={handleClick} disabled={status === 'loading'}>
        Get routes
      </button>
      {status === 'loading' && <p>Loading...</p>}
      {status === 'error' && <p role="alert">{error}</p>}
      {status === 'success' && (
        <ul>
          {routes.map((r) => (
            <li key={r.route_id}>{r.route_name}</li>
          ))}
        </ul>
      )}
    </section>
  );
}
