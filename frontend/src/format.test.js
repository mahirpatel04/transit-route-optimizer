import { describe, it, expect } from 'vitest';
import { formatDistance } from './format';

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
