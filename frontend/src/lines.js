// Official MTA subway bullet colors. Light backgrounds (yellow) use black
// text to match the real system and to stay above the 4.5:1 contrast
// minimum for small text — white-on-yellow would fail that check.
const LINE_STYLES = {
  1: { bg: '#EE352E', fg: '#FFFFFF' },
  2: { bg: '#EE352E', fg: '#FFFFFF' },
  3: { bg: '#EE352E', fg: '#FFFFFF' },
  4: { bg: '#00933C', fg: '#FFFFFF' },
  5: { bg: '#00933C', fg: '#FFFFFF' },
  6: { bg: '#00933C', fg: '#FFFFFF' },
  7: { bg: '#B933AD', fg: '#FFFFFF' },
  A: { bg: '#0039A6', fg: '#FFFFFF' },
  C: { bg: '#0039A6', fg: '#FFFFFF' },
  E: { bg: '#0039A6', fg: '#FFFFFF' },
  B: { bg: '#FF6319', fg: '#FFFFFF' },
  D: { bg: '#FF6319', fg: '#FFFFFF' },
  F: { bg: '#FF6319', fg: '#FFFFFF' },
  M: { bg: '#FF6319', fg: '#FFFFFF' },
  G: { bg: '#6CBE45', fg: '#FFFFFF' },
  J: { bg: '#996633', fg: '#FFFFFF' },
  Z: { bg: '#996633', fg: '#FFFFFF' },
  L: { bg: '#A7A9AC', fg: '#FFFFFF' },
  N: { bg: '#FCCC0A', fg: '#000000' },
  Q: { bg: '#FCCC0A', fg: '#000000' },
  R: { bg: '#FCCC0A', fg: '#000000' },
  W: { bg: '#FCCC0A', fg: '#000000' },
  S: { bg: '#808183', fg: '#FFFFFF' },
};

const DEFAULT_STYLE = { bg: '#4D4D4D', fg: '#FFFFFF' };

export function lineStyle(routeId) {
  return LINE_STYLES[routeId] || DEFAULT_STYLE;
}
