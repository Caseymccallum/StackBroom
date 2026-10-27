import { LoaderCircle, Moon, RefreshCw, Sun, X } from "lucide-react";
import type { Theme } from "../types";
import { formatWhen } from "../lib/format";
import { Button, IconButton } from "./ui";

export function Topbar({
  eyebrow,
  title,
  subtitle,
  busy,
  mode,
  lastCompleted,
  theme,
  onRefresh,
  onCancel,
  onToggleTheme,
}: {
  eyebrow: string;
  title: string;
  subtitle: string;
  busy: "quick" | "full" | null;
  mode: "quick" | "full" | null;
  lastCompleted: string | null;
  theme: Theme;
  onRefresh: () => void;
  onCancel: () => void;
  onToggleTheme: () => void;
}) {
  return (
    <header className="topbar">
      <div>
        <div className="page-eyebrow">
          <span className="dot" aria-hidden="true" />
          {eyebrow}
        </div>
        <h1 className="page-title" id="page-title" tabIndex={-1}>{title}</h1>
        <p className="page-subtitle">{subtitle}</p>
      </div>
      <div className="topbar-actions">
        <div className="status-pill" aria-live="polite">
          {busy ? <LoaderCircle size={13} className="spin" aria-hidden="true" /> : <span className="dot" aria-hidden="true" />}
          {busy
            ? busy === "full"
              ? "Full scan running"
              : "Quick scan running"
            : lastCompleted
              ? `${mode === "full" ? "Full" : "Quick"} scan · ${formatWhen(lastCompleted)}`
              : "Waiting for first scan"}
        </div>
        {busy ? (
          <Button variant="ghost" onClick={onCancel} title="Cancel the active scan">
            <X size={14} aria-hidden="true" />
            Cancel
          </Button>
        ) : null}
        <Button variant="primary" onClick={onRefresh} disabled={busy !== null} title="Refresh observations">
          <RefreshCw size={14} aria-hidden="true" />
          Refresh
        </Button>
        <IconButton
          onClick={onToggleTheme}
          aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
          title={theme === "dark" ? "Light theme" : "Dark theme"}
        >
          {theme === "dark" ? <Sun size={16} aria-hidden="true" /> : <Moon size={16} aria-hidden="true" />}
        </IconButton>
      </div>
    </header>
  );
}
