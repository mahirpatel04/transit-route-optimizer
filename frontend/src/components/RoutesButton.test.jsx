import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, afterEach } from 'vitest';
import RoutesButton from './RoutesButton';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('RoutesButton', () => {
  it('fetches and displays routes on click', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([
        { route_id: 'A', route_name: '8 Avenue Express', route_type: 1 },
      ]),
    }));

    render(<RoutesButton />);
    await userEvent.click(screen.getByRole('button', { name: /get routes/i }));

    await waitFor(() => {
      expect(screen.getByText(/8 Avenue Express/)).toBeInTheDocument();
    });
  });

  it('shows an inline error on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      json: () => Promise.resolve({ error: 'internal error' }),
    }));

    render(<RoutesButton />);
    await userEvent.click(screen.getByRole('button', { name: /get routes/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/internal error/);
    });
  });
});
