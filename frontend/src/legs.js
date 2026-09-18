// A cross-line transfer lands on a station's parent node, then a second,
// free (zero-duration) walk leg hops down to the specific child platform
// needed to board — e.g. "Walk to 635" then "Walk to 635S". Both legs are
// real graph edges and the timing is correct, but showing them as two
// separate rows reads as a nonsensical "walk to the same place twice" to a
// person. Collapse a zero-duration walk into the walk immediately before
// it, keeping only the final (real, boardable) destination.
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
