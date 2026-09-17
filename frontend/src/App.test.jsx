import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import App from './App';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('App', () => {
  it('renders the header and both panels', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ last_fetch_time: '2026-09-17T18:00:28Z' }),
    }));

    render(<App />);

    expect(screen.getByText(/last updated/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /get routes/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /find nearby stops/i })).toBeInTheDocument();
  });
});
