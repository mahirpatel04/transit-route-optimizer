import { describe, it, expect } from 'vitest';
import { collapseAdjacentWalks } from './legs';

describe('collapseAdjacentWalks', () => {
  it('merges a zero-duration walk into the walk before it', () => {
    const legs = [
      { kind: 'walk', from_stop_id: 'X', to_stop_id: '635', depart_at: 't0', arrive_at: 't1' },
      { kind: 'walk', from_stop_id: '635', to_stop_id: '635S', depart_at: 't1', arrive_at: 't1' },
      { kind: 'ride', route_id: '4', from_stop_id: '635S', to_stop_id: '640S', depart_at: 't1', arrive_at: 't2' },
    ];

    const got = collapseAdjacentWalks(legs);

    expect(got).toHaveLength(2);
    expect(got[0]).toMatchObject({ kind: 'walk', to_stop_id: '635S', depart_at: 't0', arrive_at: 't1' });
    expect(got[1]).toMatchObject({ kind: 'ride', from_stop_id: '635S' });
  });

  it('leaves legs untouched when no zero-duration walks are present', () => {
    const legs = [
      { kind: 'ride', route_id: 'N', from_stop_id: 'R15S', to_stop_id: 'R16S', depart_at: 't0', arrive_at: 't1' },
      { kind: 'walk', from_stop_id: 'R16S', to_stop_id: '902', depart_at: 't1', arrive_at: 't2' },
    ];

    expect(collapseAdjacentWalks(legs)).toEqual(legs);
  });

  it('keeps a zero-duration walk that is not preceded by another walk', () => {
    const legs = [
      { kind: 'ride', route_id: '4', from_stop_id: 'A', to_stop_id: 'B', depart_at: 't0', arrive_at: 't1' },
      { kind: 'walk', from_stop_id: 'B', to_stop_id: 'BS', depart_at: 't1', arrive_at: 't1' },
    ];

    expect(collapseAdjacentWalks(legs)).toEqual(legs);
  });

  it('collapses more than two consecutive zero-duration walks into one', () => {
    const legs = [
      { kind: 'walk', from_stop_id: 'X', to_stop_id: '635', depart_at: 't0', arrive_at: 't1' },
      { kind: 'walk', from_stop_id: '635', to_stop_id: '635S', depart_at: 't1', arrive_at: 't1' },
      { kind: 'walk', from_stop_id: '635S', to_stop_id: '635SS', depart_at: 't1', arrive_at: 't1' },
    ];

    const got = collapseAdjacentWalks(legs);

    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ to_stop_id: '635SS', depart_at: 't0', arrive_at: 't1' });
  });
});
