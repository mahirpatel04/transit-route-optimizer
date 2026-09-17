import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, afterEach } from 'vitest';
import StopsNearForm from './StopsNearForm';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('StopsNearForm', () => {
  it('fetches and displays nearby stops for valid input', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([
        { stop_id: '902', stop_name: 'Times Sq-42 St', lat: 40.756, lon: -73.986, distance_meters: 232.5 },
      ]),
    }));

    render(<StopsNearForm />);
    await userEvent.type(screen.getByLabelText(/latitude/i), '40.758');
    await userEvent.type(screen.getByLabelText(/longitude/i), '-73.9855');
    await userEvent.click(screen.getByRole('button', { name: /find nearby stops/i }));

    await waitFor(() => {
      expect(screen.getByText(/Times Sq-42 St/)).toBeInTheDocument();
    });
  });

  it('shows a validation error for out-of-range input without calling the API', async () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);

    render(<StopsNearForm />);
    await userEvent.type(screen.getByLabelText(/latitude/i), '999');
    await userEvent.type(screen.getByLabelText(/longitude/i), '0');
    await userEvent.click(screen.getByRole('button', { name: /find nearby stops/i }));

    expect(screen.getByRole('alert')).toHaveTextContent(/valid latitude/i);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('shows an inline error when the API returns one', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      json: () => Promise.resolve({ error: 'invalid or missing lat/lon' }),
    }));

    render(<StopsNearForm />);
    await userEvent.type(screen.getByLabelText(/latitude/i), '40.758');
    await userEvent.type(screen.getByLabelText(/longitude/i), '-73.9855');
    await userEvent.click(screen.getByRole('button', { name: /find nearby stops/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid or missing lat\/lon/);
    });
  });
});
