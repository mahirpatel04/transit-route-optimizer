import { describe, it, expect } from 'vitest';
import { lineStyle } from './lines';

describe('lineStyle', () => {
  it('returns white text on red for the 1/2/3 lines', () => {
    expect(lineStyle('1')).toEqual({ bg: '#EE352E', fg: '#FFFFFF' });
  });

  it('returns black text for the yellow N/Q/R/W lines (contrast)', () => {
    expect(lineStyle('N')).toEqual({ bg: '#FCCC0A', fg: '#000000' });
  });

  it('falls back to a neutral style for an unknown route id', () => {
    expect(lineStyle('XYZ')).toEqual({ bg: '#4D4D4D', fg: '#FFFFFF' });
  });
});
