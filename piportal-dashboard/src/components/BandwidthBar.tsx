function formatBytes(bytes: number): string {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB';
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + ' MB';
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return bytes + ' B';
}

export default function BandwidthBar({ used, limit }: { used: number; limit: number }) {
  const pct = limit > 0 ? Math.min((used / limit) * 100, 100) : 0;
  const warn = pct > 80;

  return (
    <div className="bandwidth">
      <div className="bandwidth-bar">
        <div
          className={`bandwidth-fill ${warn ? 'bandwidth-warn' : ''}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="bandwidth-label">
        {formatBytes(used)} / {formatBytes(limit)}
      </span>
    </div>
  );
}
