import { lineStyle } from '../lines';

export default function RouteBullet({ routeId }) {
  const { bg, fg } = lineStyle(routeId);
  return (
    <span
      className="route-bullet"
      style={{ backgroundColor: bg, color: fg }}
      aria-hidden="true"
    >
      {routeId}
    </span>
  );
}
