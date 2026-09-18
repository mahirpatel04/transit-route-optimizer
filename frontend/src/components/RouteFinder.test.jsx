import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, afterEach } from 'vitest';
import RouteFinder from './RouteFinder';

afterEach(() => {
  vi.unstubAllGlobals();
});

async function fillAndSubmit(from, to) {
  if (from) await userEvent.type(screen.getByLabelText(/^from$/i), from);
  if (to) await userEvent.type(screen.getByLabelText(/^to$/i), to);
  await userEvent.click(screen.getByRole('button', { name: /find route/i }));
}

describe('RouteFinder', () => {
  it('fetches and displays a route on submit', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        total_time_seconds: 540,
        legs: [
          {
            kind: 'walk',
            route_id: '',
            from_stop_id: '',
            from_stop_name: '',
            to_stop_id: 'R15S',
            to_stop_name: 'Times Sq-42 St',
            depart_at: '2026-09-17T23:05:00-04:00',
            arrive_at: '2026-09-17T23:08:00-04:00',
          },
          {
            kind: 'ride',
            route_id: 'N',
            from_stop_id: 'R15S',
            from_stop_name: 'Times Sq-42 St',
            to_stop_id: 'R16S',
            to_stop_name: '5 Av-59 St',
            depart_at: '2026-09-17T23:08:00-04:00',
            arrive_at: '2026-09-17T23:09:30-04:00',
          },
          {
            kind: 'walk',
            route_id: '',
            from_stop_id: 'R16S',
            from_stop_name: '5 Av-59 St',
            to_stop_id: '902',
            to_stop_name: 'Grand Central-42 St',
            depart_at: '2026-09-17T23:09:30-04:00',
            arrive_at: '2026-09-17T23:12:30-04:00',
          },
          {
            kind: 'walk',
            route_id: '',
            from_stop_id: '902',
            from_stop_name: 'Grand Central-42 St',
            to_stop_id: '',
            to_stop_name: 'Grand Central Terminal, NYC',
            depart_at: '2026-09-17T23:12:30-04:00',
            arrive_at: '2026-09-17T23:14:00-04:00',
          },
        ],
      }),
    }));

    render(<RouteFinder />);
    await fillAndSubmit('Times Square, NYC', 'Grand Central Terminal, NYC');

    expect(await screen.findByText(/total time: 9 min/i)).toBeInTheDocument();
    expect(screen.getByText('N')).toBeInTheDocument();
    expect(screen.getByText(/take the n train from times sq-42 st to 5 av-59 st/i)).toBeInTheDocument();
    expect(screen.getByText(/walk to times sq-42 st/i)).toBeInTheDocument();
    expect(screen.getByText(/walk to grand central-42 st/i)).toBeInTheDocument();
    expect(screen.getByText(/walk to grand central terminal, nyc/i)).toBeInTheDocument();
  });

  it('omits the optimize param by default and includes it when checked', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ total_time_seconds: 60, legs: [] }),
    }));

    render(<RouteFinder />);
    await fillAndSubmit('Times Square, NYC', 'Grand Central Terminal, NYC');
    expect(fetch).toHaveBeenLastCalledWith(expect.not.stringContaining('optimize'));

    await userEvent.click(screen.getByRole('checkbox', { name: /optimize for fastest trip/i }));
    await fillAndSubmit();
    expect(fetch).toHaveBeenLastCalledWith(expect.stringContaining('optimize=true'));
  });

  it('fills the corresponding input when a suggestion chip is clicked', async () => {
    render(<RouteFinder />);

    const fromChips = screen.getByRole('group', { name: /suggested starting points/i });
    await userEvent.click(within(fromChips).getByRole('button', { name: /times square/i }));
    expect(screen.getByLabelText(/^from$/i)).toHaveValue('Times Square, NYC');

    const toChips = screen.getByRole('group', { name: /suggested destinations/i });
    await userEvent.click(within(toChips).getByRole('button', { name: /grand central terminal/i }));
    expect(screen.getByLabelText(/^to$/i)).toHaveValue('Grand Central Terminal, NYC');
  });

  it('requires both from and to before submitting', async () => {
    vi.stubGlobal('fetch', vi.fn());

    render(<RouteFinder />);
    await fillAndSubmit('Times Square, NYC', '');

    expect(screen.getByRole('alert')).toHaveTextContent(/enter both/i);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('shows an inline error on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      json: () => Promise.resolve({ error: "could not find 'from' address" }),
    }));

    render(<RouteFinder />);
    await fillAndSubmit('nowhere', 'Grand Central Terminal, NYC');

    expect(await screen.findByRole('alert')).toHaveTextContent(/could not find/i);
  });

  it('shows an inline error on a network failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));

    render(<RouteFinder />);
    await fillAndSubmit('Times Square, NYC', 'Grand Central Terminal, NYC');

    expect(await screen.findByRole('alert')).toHaveTextContent(/network error/i);
  });
});
