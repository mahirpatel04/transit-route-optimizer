// A cross-line transfer lands on a station's parent node, then a free
// zero-duration walk hops to the boarding platform — e.g. "Walk to 635"
// then "Walk to 635S". Collapse the zero-duration walk into the one before
// it so it doesn't render as two redundant rows.
export function collapseAdjacentWalks(legs) {
  const collapsed = [];
  for (const leg of legs) {
    const isZeroDurationWalk = leg.kind === 'walk' && leg.depart_at === leg.arrive_at;
    const previous = collapsed[collapsed.length - 1];
    if (isZeroDurationWalk && previous?.kind === 'walk') {
      collapsed[collapsed.length - 1] = { ...previous, to_stop_id: leg.to_stop_id, to_stop_name: leg.to_stop_name };
    } else {
      collapsed.push(leg);
    }
  }
  return collapsed;
}
