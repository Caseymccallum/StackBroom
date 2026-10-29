import { HardDrive, LayoutDashboard, List, Settings, ShieldCheck, Sparkles, Wrench } from "lucide-react";
import type { NavKey } from "../types";

const ITEMS: Array<{
  key: NavKey;
  label: string;
  hint: string;
  shortcut: string;
  icon: typeof LayoutDashboard;
}> = [
  { key: "overview", label: "Overview", hint: "This PC", shortcut: "Ctrl 1", icon: LayoutDashboard },
  { key: "processes", label: "Processes", hint: "Memory & roles", shortcut: "Ctrl 2", icon: List },
  { key: "storage", label: "Storage", hint: "Volumes", shortcut: "Ctrl 3", icon: HardDrive },
  { key: "cleaning", label: "Cleaning", hint: "Find & review", shortcut: "Ctrl 4", icon: Sparkles },
  { key: "developer", label: "Developer Tools", hint: "Docker · WSL · TCP", shortcut: "Ctrl 5", icon: Wrench },
  { key: "settings", label: "Settings", hint: "Privacy & display", shortcut: "Ctrl 6", icon: Settings },
];

export function Sidebar({
  active,
  showDeveloperTools,
  version,
  onNavigate,
}: {
  active: NavKey;
  showDeveloperTools: boolean;
  version: string;
  onNavigate: (key: NavKey) => void;
}) {
  const items = ITEMS.filter((item) => item.key !== "developer" || showDeveloperTools);
  return (
    <aside className="sidebar" aria-label="Primary navigation">
      <div className="brand">
        <div className="brand-mark" aria-hidden="true">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
            <path d="M5 5.5h14M6.5 9.5h11M8 13.5h8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            <path d="M15.5 15.5l4 3.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            <path d="M13 17.5l2.5-2 2.5 2.2-2.5 2z" fill="currentColor" opacity=".9" />
          </svg>
        </div>
        <div className="brand-copy">
          <div className="brand-name">StackBroom</div>
          <div className="brand-sub">Your PC, understood</div>
        </div>
      </div>

      <div className="nav-label">Workspace</div>
      <nav className="nav-list">
        {items.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.key}
              type="button"
              className="nav-item"
              aria-label={item.label}
              aria-current={active === item.key ? "page" : undefined}
              title={`${item.label} (${item.shortcut})`}
              onClick={() => onNavigate(item.key)}
            >
              <Icon aria-hidden="true" />
              <span className="nav-item-text">
                <span className="nav-item-title">{item.label}</span>
                <span className="nav-item-hint">{item.hint}</span>
              </span>
            </button>
          );
        })}
      </nav>

      <div className="sidebar-footer">
        <div className="readonly-chip">
          <ShieldCheck size={15} aria-hidden="true" />
          <span>Verified control</span>
        </div>
        <div className="sidebar-version">
          <span>v{version}</span>
          <span>Local only</span>
        </div>
      </div>
    </aside>
  );
}
