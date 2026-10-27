import { LockKeyhole, Monitor, ShieldCheck, Sparkles } from "lucide-react";
import type { AppInfo } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import type { Settings } from "../types";
import { Badge, Card, CardBody, CardHeader, Segmented, Toggle } from "../components/ui";

export function SettingsPage({
  settings,
  onChange,
  appInfo,
}: {
  settings: Settings;
  onChange: (next: Settings) => void;
  appInfo: AppInfo | null;
}) {
  return (
    <div className="settings-grid">
      <div className="grid" style={{ gap: 16 }}>
        <Card>
          <CardHeader title="Appearance" subtitle="Dark-first, with a full light theme." />
          <CardBody>
            <div className="setting-row">
              <div>
                <div className="setting-name">Theme</div>
                <div className="setting-note">Saved in local browser storage only. No account or cloud sync.</div>
              </div>
              <Segmented
                value={settings.theme}
                label="Theme"
                options={[
                  { value: "dark", label: "Dark" },
                  { value: "light", label: "Light" },
                ]}
                onChange={(theme) => onChange({ ...settings, theme })}
              />
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardHeader title="Refresh" subtitle="Fast local metrics and slower developer inspection are separate." />
          <CardBody>
            <div className="setting-row">
              <div>
                <div className="setting-name">Automatic quick refresh</div>
                <div className="setting-note">
                  Off by default. Only reads local memory, CPU, volume and process counters. Docker and WSL are never
                  run on a timer.
                </div>
              </div>
              <select
                className="select"
                aria-label="Automatic quick refresh interval"
                value={settings.autoRefreshSeconds}
                onChange={(event) => onChange({ ...settings, autoRefreshSeconds: Number(event.target.value) })}
              >
                <option value={0}>Off</option>
                <option value={30}>Every 30 seconds</option>
                <option value={60}>Every minute</option>
                <option value={120}>Every 2 minutes</option>
              </select>
            </div>
            <div className="setting-row">
              <div>
                <div className="setting-name">Developer Tools visibility</div>
                <div className="setting-note">
                  Show Docker, WSL, TCP listener and duplicate-server evidence in the sidebar. Hiding the screen does not
                  disable any inspection.
                </div>
              </div>
              <Toggle
                checked={settings.showDeveloperTools}
                label="Show Developer Tools"
                onChange={(showDeveloperTools) => onChange({ ...settings, showDeveloperTools })}
              />
            </div>
          </CardBody>
        </Card>
        <Card>
          <CardHeader title="Privacy and boundaries" subtitle="The same policy as the CLI, applied before data reaches the interface." />
          <CardBody>
            <div style={{ display: "grid", gap: 12 }}>
              <div className="banner"><ShieldCheck size={16} aria-hidden="true" /><div><strong style={{ color: "var(--text)" }}>Read-only.</strong> No process termination, container removal, prune, file deletion or settings mutation exists in this build.</div></div>
              <div className="banner"><LockKeyhole size={16} aria-hidden="true" /><div><strong style={{ color: "var(--text)" }}>Command lines stay out of the UI.</strong> Executable paths, project roots and profile paths are withheld or tokenised. Diagnostics mode is not exposed.</div></div>
              <div className="banner"><Monitor size={16} aria-hidden="true" /><div><strong style={{ color: "var(--text)" }}>No listening port.</strong> The packaged application serves embedded assets to its own WebView2 instance. It does not open a network server.</div></div>
              <div className="banner"><Sparkles size={16} aria-hidden="true" /><div><strong style={{ color: "var(--text)" }}>No AI provider.</strong> A future BYOK adviser is design-only; no credentials are requested or stored and no external request is made.</div></div>
            </div>
          </CardBody>
        </Card>
      </div>

      <div className="grid" style={{ gap: 16 }}>
        <Card>
          <CardHeader title="About StackBroom" action={<Badge tone="teal">Read-only</Badge>} />
          <CardBody>
            <div className="setting-row"><span className="muted">Version</span><span className="mono">{appInfo?.version || "Unknown"}</span></div>
            <div className="setting-row"><span className="muted">Engine schema</span><span className="mono">{appInfo?.schemaVersion || "Unknown"}</span></div>
            <div className="setting-row"><span className="muted">Toolchain</span><span className="mono">{appInfo?.toolchain || "Unknown"}</span></div>
            <div className="setting-row"><span className="muted">Desktop bridge</span><span>Wails v3 · React · TypeScript</span></div>
          </CardBody>
        </Card>

        <Card>
          <CardHeader title="Known limitations" subtitle="What this milestone deliberately does not claim." />
          <CardBody>
            <ul className="limitations">
              {(appInfo?.limitations ?? ["Application information is unavailable."]).map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </CardBody>
        </Card>

        <Card>
          <CardHeader title="Validation note" subtitle="Go toolchain security." />
          <CardBody>
            <p className="setting-note" style={{ maxWidth: "none", margin: 0 }}>
              This build is compiled with the installed Go toolchain reported above. The project baseline was Go 1.27.0;
              Go 1.27.3 contains a security update. No system-wide toolchain was changed during this milestone. Release
              builds should be produced with Go 1.27.3 or later after the owner validates it.
            </p>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
