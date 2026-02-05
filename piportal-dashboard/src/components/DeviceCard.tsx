import { Link } from 'react-router-dom';
import type { DeviceInfo } from '../api';
import StatusBadge from './StatusBadge';
import BandwidthBar from './BandwidthBar';

function timeAgo(dateStr?: string): string {
  if (!dateStr) return 'Never';
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'Just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function DeviceCard({ device }: { device: DeviceInfo }) {
  const hasMetrics = device.is_online && device.mem_total != null && device.mem_total > 0;

  return (
    <Link to={`/dashboard/devices/${device.id}`} className="device-card">
      <div className="device-card-header">
        <span className="device-subdomain">{device.subdomain}</span>
        <StatusBadge online={device.is_online} />
      </div>
      <div className="device-card-url">{device.url}</div>
      <div className="device-card-meta">
        Last seen: {timeAgo(device.last_seen_at)}
        {device.is_online && !device.tunnel_enabled && (
          <span className="forwarding-off">Forwarding off</span>
        )}
      </div>
      {hasMetrics && (
        <div className="device-card-metrics">
          {device.cpu_temp != null && device.cpu_temp >= 0 && (
            <span>{device.cpu_temp.toFixed(0)}&deg;C</span>
          )}
          <span>{Math.round(((device.mem_total! - device.mem_free!) / device.mem_total!) * 100)}% mem</span>
          {device.disk_total != null && device.disk_total > 0 && (
            <span>{Math.round(((device.disk_total! - device.disk_free!) / device.disk_total!) * 100)}% disk</span>
          )}
        </div>
      )}
      <BandwidthBar used={device.bytes_total} limit={device.limit} />
    </Link>
  );
}
