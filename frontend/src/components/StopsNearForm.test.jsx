import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, afterEach } from 'vitest';
import StopsNearForm from './StopsNearForm';

afterEach(() => {
  vi.unstubAllGlobals();
  delete global.navigator.geolocation;
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

  it('shows an inline error on a network failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));

    render(<StopsNearForm />);
    await userEvent.type(screen.getByLabelText(/latitude/i), '40.758');
    await userEvent.type(screen.getByLabelText(/longitude/i), '-73.9855');
    await userEvent.click(screen.getByRole('button', { name: /find nearby stops/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/network error/i);
    });
  });

  it('shows an empty state when no stops are found', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    }));

    render(<StopsNearForm />);
    await userEvent.type(screen.getByLabelText(/latitude/i), '40.758');
    await userEvent.type(screen.getByLabelText(/longitude/i), '-73.9855');
    await userEvent.click(screen.getByRole('button', { name: /find nearby stops/i }));

    await waitFor(() => {
      expect(screen.getByText(/no stops found/i)).toBeInTheDocument();
    });
  });

  it('fills lat/lon from the browser geolocation API', async () => {
    global.navigator.geolocation = {
      getCurrentPosition: vi.fn((success) => {
        success({ coords: { latitude: 40.758, longitude: -73.9855 } });
      }),
    };

    render(<StopsNearForm />);
    await userEvent.click(screen.getByRole('button', { name: /use my location/i }));

    expect(screen.getByLabelText(/latitude/i)).toHaveValue(40.758);
    expect(screen.getByLabelText(/longitude/i)).toHaveValue(-73.9855);
  });

  it('shows an inline error when geolocation is denied', async () => {
    global.navigator.geolocation = {
      getCurrentPosition: vi.fn((_success, error) => {
        error(new Error('denied'));
      }),
    };

    render(<StopsNearForm />);
    await userEvent.click(screen.getByRole('button', { name: /use my location/i }));

    expect(screen.getByRole('alert')).toHaveTextContent(/could not get your location/i);
  });
});
