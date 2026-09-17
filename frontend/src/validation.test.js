import { describe, it, expect } from 'vitest';
import { isValidLatLon } from './validation';

describe('isValidLatLon', () => {
  it('accepts a valid coordinate', () => {
    expect(isValidLatLon(40.758, -73.9855)).toBe(true);
  });

  it('accepts boundary values', () => {
    expect(isValidLatLon(90, 180)).toBe(true);
    expect(isValidLatLon(-90, -180)).toBe(true);
  });

  it('rejects out-of-range latitude', () => {
    expect(isValidLatLon(999, 0)).toBe(false);
  });

  it('rejects out-of-range longitude', () => {
    expect(isValidLatLon(0, 999)).toBe(false);
  });

  it('rejects NaN', () => {
    expect(isValidLatLon(NaN, 0)).toBe(false);
    expect(isValidLatLon(0, NaN)).toBe(false);
  });

  it('rejects Infinity', () => {
    expect(isValidLatLon(Infinity, 0)).toBe(false);
  });
});
