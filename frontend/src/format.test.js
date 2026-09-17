import { describe, it, expect } from 'vitest';
import { formatDistance, formatRelativeTime } from './format';

describe('formatDistance', () => {
  it('shows meters, rounded, under 1km', () => {
    expect(formatDistance(232.5)).toBe('233 m');
  });

  it('shows kilometers with one decimal at or above 1km', () => {
    expect(formatDistance(1500)).toBe('1.5 km');
  });

  it('shows meters right up to the 1km boundary', () => {
    expect(formatDistance(999)).toBe('999 m');
  });
});

describe('formatRelativeTime', () => {
  const now = new Date('2026-09-17T18:00:00Z');

  it('shows "just now" under a minute ago', () => {
    expect(formatRelativeTime('2026-09-17T17:59:30Z', now)).toBe('just now');
  });

  it('shows minutes ago', () => {
    expect(formatRelativeTime('2026-09-17T17:55:00Z', now)).toBe('5 minutes ago');
  });

  it('uses singular "minute"', () => {
    expect(formatRelativeTime('2026-09-17T17:59:00Z', now)).toBe('1 minute ago');
  });

  it('shows hours ago', () => {
    expect(formatRelativeTime('2026-09-17T15:00:00Z', now)).toBe('3 hours ago');
  });

  it('uses singular "hour"', () => {
    expect(formatRelativeTime('2026-09-17T17:00:00Z', now)).toBe('1 hour ago');
  });

  it('shows days ago', () => {
    expect(formatRelativeTime('2026-09-15T18:00:00Z', now)).toBe('2 days ago');
  });

  it('falls back to a plain date at 7+ days', () => {
    expect(formatRelativeTime('2026-09-01T18:00:00Z', now)).toBe(
      new Date('2026-09-01T18:00:00Z').toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })
    );
  });

  it('returns the raw input for an unparseable timestamp instead of throwing', () => {
    expect(formatRelativeTime('not-a-date', now)).toBe('not-a-date');
  });
});
