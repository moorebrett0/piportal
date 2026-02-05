import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { api, type DeviceInfo, type OrgInfo } from '../api';
import DeviceCard from '../components/DeviceCard';

export default function DashboardPage() {
  const [searchParams] = useSearchParams();
  const orgId = searchParams.get('org_id');

  const [devices, setDevices] = useState<DeviceInfo[]>([]);
  const [orgName, setOrgName] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    setError('');

    const fetchData = async () => {
      try {
        const deviceList = await api.listDevices(orgId || undefined);
        setDevices(deviceList);

        // Get org name if filtering by org
        if (orgId) {
          const orgs = await api.listOrgs();
          const org = orgs.find((o: OrgInfo) => o.id === orgId);
          setOrgName(org?.name || '');
        } else {
          setOrgName('');
        }
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [orgId]);

  if (loading) return <div className="loading">Loading devices...</div>;
  if (error) return <div className="error-msg">{error}</div>;

  const addDeviceLink = orgId ? `/dashboard/add?org_id=${orgId}` : '/dashboard/add';
  const pageTitle = orgName ? orgName : 'Your Devices';

  return (
    <div className="dashboard-page">
      <div className="page-header">
        <h1>{pageTitle}</h1>
        <Link to={addDeviceLink} className="btn">Add Device</Link>
      </div>
      {devices.length === 0 ? (
        <div className="empty-state">
          <p>No devices {orgName ? `in ${orgName}` : 'yet'}.</p>
          <p>
            <Link to={addDeviceLink}>Create a new device</Link> or claim an existing one with its token.
          </p>
        </div>
      ) : (
        <div className="device-grid">
          {devices.map(d => <DeviceCard key={d.id} device={d} />)}
        </div>
      )}
    </div>
  );
}
