import { useState, useEffect } from 'react';
import { Link, Outlet, useNavigate, useLocation, useSearchParams } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { api, type OrgInfo } from '../api';

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const currentOrgId = searchParams.get('org_id');

  const [orgs, setOrgs] = useState<OrgInfo[]>([]);
  const [showCreateInput, setShowCreateInput] = useState(false);
  const [newOrgName, setNewOrgName] = useState('');
  const [editingOrgId, setEditingOrgId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (user) {
      api.listOrgs().then(setOrgs).catch(() => {});
    }
  }, [user]);

  const handleLogout = async () => {
    await logout();
    navigate('/dashboard/login');
  };

  const isActive = (path: string) => location.pathname === path && !currentOrgId;
  const isOrgActive = (orgId: string) => currentOrgId === orgId;

  const handleCreateOrg = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newOrgName.trim() || creating) return;
    setCreating(true);
    try {
      const org = await api.createOrg(newOrgName.trim());
      setOrgs([...orgs, org].sort((a, b) => a.name.localeCompare(b.name)));
      setNewOrgName('');
      setShowCreateInput(false);
    } catch (err) {
      console.error('Failed to create org:', err);
    }
    setCreating(false);
  };

  const handleRenameOrg = async (orgId: string) => {
    if (!editingName.trim()) {
      setEditingOrgId(null);
      return;
    }
    try {
      await api.updateOrg(orgId, editingName.trim());
      setOrgs(orgs.map(o => o.id === orgId ? { ...o, name: editingName.trim() } : o)
        .sort((a, b) => a.name.localeCompare(b.name)));
    } catch (err) {
      console.error('Failed to rename org:', err);
    }
    setEditingOrgId(null);
  };

  const handleDeleteOrg = async (orgId: string, orgName: string) => {
    if (!confirm(`Delete "${orgName}"? Devices with this tag will become untagged.`)) return;
    try {
      await api.deleteOrg(orgId);
      setOrgs(orgs.filter(o => o.id !== orgId));
      if (currentOrgId === orgId) {
        navigate('/dashboard');
      }
    } catch (err) {
      console.error('Failed to delete org:', err);
    }
  };

  const startEditing = (org: OrgInfo) => {
    setEditingOrgId(org.id);
    setEditingName(org.name);
  };

  // Auth pages (login/signup) — no sidebar
  if (!user) {
    return (
      <div className="layout auth-layout">
        <main className="main">
          <Outlet />
        </main>
      </div>
    );
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <Link to="/dashboard" className="sidebar-logo">PiPortal</Link>
        <div className="sidebar-divider" />
        <nav className="sidebar-nav">
          <Link to="/dashboard" className={isActive('/dashboard') ? 'active' : ''}>
            <span className="sidebar-nav-icon">▣</span>
            All Devices
          </Link>
          <Link to="/dashboard/add" className={isActive('/dashboard/add') ? 'active' : ''}>
            <span className="sidebar-nav-icon">+</span>
            Add Device
          </Link>
        </nav>

        <div className="sidebar-orgs">
          <div className="sidebar-orgs-header">
            <span>Tags</span>
            <button
              className="sidebar-orgs-add"
              onClick={() => setShowCreateInput(!showCreateInput)}
              title="Create tag"
            >
              +
            </button>
          </div>
          {showCreateInput && (
            <form onSubmit={handleCreateOrg} className="sidebar-org-create">
              <input
                type="text"
                value={newOrgName}
                onChange={e => setNewOrgName(e.target.value)}
                placeholder="Tag name"
                autoFocus
                maxLength={50}
              />
              <button type="submit" disabled={creating || !newOrgName.trim()}>
                {creating ? '...' : 'Add'}
              </button>
            </form>
          )}
          <div className="sidebar-orgs-list">
            {orgs.map(org => (
              <div key={org.id} className={`sidebar-org-item ${isOrgActive(org.id) ? 'active' : ''}`}>
                {editingOrgId === org.id ? (
                  <input
                    type="text"
                    value={editingName}
                    onChange={e => setEditingName(e.target.value)}
                    onBlur={() => handleRenameOrg(org.id)}
                    onKeyDown={e => {
                      if (e.key === 'Enter') handleRenameOrg(org.id);
                      if (e.key === 'Escape') setEditingOrgId(null);
                    }}
                    className="sidebar-org-edit-input"
                    autoFocus
                    maxLength={50}
                  />
                ) : (
                  <>
                    <Link
                      to={`/dashboard?org_id=${org.id}`}
                      className="sidebar-org-link"
                      onDoubleClick={() => startEditing(org)}
                    >
                      {org.name}
                    </Link>
                    <button
                      className="sidebar-org-delete"
                      onClick={() => handleDeleteOrg(org.id, org.name)}
                      title="Delete tag"
                    >
                      ×
                    </button>
                  </>
                )}
              </div>
            ))}
            {orgs.length === 0 && !showCreateInput && (
              <div className="sidebar-orgs-empty">No tags yet</div>
            )}
          </div>
        </div>

        <div className="sidebar-footer">
          <span className="sidebar-email">{user.email}</span>
          <button onClick={handleLogout} className="sidebar-logout">
            Log out
          </button>
        </div>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
