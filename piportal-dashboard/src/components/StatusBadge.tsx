export default function StatusBadge({ online }: { online: boolean }) {
  return (
    <span className={`badge ${online ? 'badge-online' : 'badge-offline'}`}>
      {online ? 'Online' : 'Offline'}
    </span>
  );
}
