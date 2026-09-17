import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import Header from './Header';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('Header', () => {
  it('shows the last fetch time on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ last_fetch_time: '2026-09-17T18:00:28Z' }),
    }));

    render(<Header />);

    await waitFor(() => {
      expect(screen.getByText(/2026-09-17T18:00:28Z/)).toBeInTheDocument();
    });
  });

  it('falls back to an em dash on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));

    render(<Header />);

    await waitFor(() => {
      expect(screen.getByText(/—/)).toBeInTheDocument();
    });
  });
});
