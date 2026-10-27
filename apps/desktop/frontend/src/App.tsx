import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { useDesktop } from "./hooks/useDesktop";
import type { NavKey, Settings } from "./types";
import { DEFAULT_SETTINGS } from "./types";
import { Sidebar } from "./components/Sidebar";
import { Topbar } from "./components/Topbar";
import { Button, Notice } from "./components/ui";
import { OverviewPage } from "./pages/Overview";
import { ProcessesPage } from "./pages/Processes";
import { StoragePage } from "./pages/Storage";
import { DeveloperPage } from "./pages/Developer";
import { SettingsPage } from "./pages/Settings";

const SETTINGS_KEY = "stackbroom.desktop.settings.v1";
const VALID_NAV: NavKey[] = ["overview", "processes", "storage", "developer", "settings"];

function loadSettings(): Settings {
  try {
    const raw = window.localStorage.getItem(SETTINGS_KEY);
    if (!raw) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(raw) as Partial<Settings>;
    return {
      theme: parsed.theme === "light" ? "light" : "dark",
      autoRefreshSeconds: [0, 30, 60, 120].includes(Number(parsed.autoRefreshSeconds))
        ? Number(parsed.autoRefreshSeconds)
        : 0,
      showDeveloperTools: parsed.showDeveloperTools !== false,
    };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

function navFromHash(): NavKey {
  const value = window.location.hash.replace(/^#\/?/, "") as NavKey;
  return VALID_NAV.includes(value) ? value : "overview";
}

const PAGE_META: Record<NavKey, { eyebrow: string; title: string; subtitle: string }> = {
  overview: {
    eyebrow: "System overview",
    title: "This PC at a glance",
    subtitle: "Real memory, CPU, storage and process observations from Windows. No scores, no invented health ratings.",
  },
  processes: {
    eyebrow: "Process inventory",
    title: "Processes",
    subtitle: "Search, sort and filter observed processes. Unknown values stay unknown, and there is no kill control.",
  },
  storage: {
    eyebrow: "Storage",
    title: "Volume capacity",
    subtitle: "A first-stage read-only view of fixed-volume capacity and free space—not a reclaimable-space promise.",
  },
  developer: {
    eyebrow: "Developer evidence",
    title: "Developer Tools",
    subtitle: "Docker, WSL, TCP listener ownership and duplicate-server evidence from the existing engine.",
  },
  settings: {
    eyebrow: "Preferences",
    title: "Settings",
    subtitle: "Appearance, refresh behaviour, privacy and the limits of this milestone.",
  },
};

export default function App() {
  const [settings, setSettings] = useState<Settings>(loadSettings);
  const [active, setActive] = useState<NavKey>(navFromHash);
  const desktop = useDesktop(settings.autoRefreshSeconds);

  useEffect(() => {
    if (!desktop.appInfo?.preview) {
      window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
    }
    document.documentElement.dataset.theme = settings.theme;
    document.documentElement.style.colorScheme = settings.theme;
  }, [settings, desktop.appInfo?.preview]);

  useEffect(() => {
    if (!desktop.appInfo?.preview) return;
    setSettings({
      theme: desktop.appInfo.previewTheme === "light" ? "light" : "dark",
      autoRefreshSeconds: 0,
      showDeveloperTools: true,
    });
    const screen = desktop.appInfo.previewScreen as NavKey | undefined;
    if (screen && VALID_NAV.includes(screen)) {
      setActive(screen);
      window.history.replaceState(null, "", `#${screen}`);
    }
  }, [desktop.appInfo?.preview, desktop.appInfo?.previewTheme, desktop.appInfo?.previewScreen]);

  useEffect(() => {
    const onHashChange = () => setActive(navFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    if (!settings.showDeveloperTools && active === "developer") {
      navigate("overview");
    }
  }, [settings.showDeveloperTools, active]);

  useEffect(() => {
    const main = document.getElementById("main-content");
    if (main) {
      if (typeof main.scrollTo === "function") {
        main.scrollTo({ top: 0 });
      } else {
        main.scrollTop = 0;
      }
    }
    requestAnimationFrame(() => {
      // Put keyboard scrolling in its owning pane after navigation.
      main?.focus({ preventScroll: true });
    });
  }, [active]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!event.ctrlKey) return;
      if (event.shiftKey && (event.key === "D" || event.key === "d")) {
        event.preventDefault();
        setSettings((current) => ({ ...current, theme: "dark" }));
        return;
      }
      if (event.shiftKey && (event.key === "L" || event.key === "l")) {
        event.preventDefault();
        setSettings((current) => ({ ...current, theme: "light" }));
        return;
      }
      if (event.key === "r" || event.key === "R") {
        event.preventDefault();
        void refreshCurrent();
        return;
      }
      const index = Number(event.key);
      if (index >= 1 && index <= 5) {
        const target = VALID_NAV[index - 1];
        if (target === "developer" && !settings.showDeveloperTools) return;
        event.preventDefault();
        navigate(target);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  function navigate(target: NavKey) {
    setActive(target);
    if (window.location.hash !== `#${target}`) {
      window.history.replaceState(null, "", `#${target}`);
    }
  }

  async function refreshCurrent() {
    if (desktop.busy) return;
    await desktop.refresh(active === "processes" || active === "developer" ? "full" : "quick");
  }

  const lastCompleted = useMemo(() => {
    const quickAt = desktop.quick ? new Date(desktop.quick.completedAt).getTime() : 0;
    const fullAt = desktop.full ? new Date(desktop.full.completedAt).getTime() : 0;
    if (fullAt >= quickAt && desktop.full) return desktop.full.completedAt;
    return desktop.quick?.completedAt ?? null;
  }, [desktop.quick, desktop.full]);

  const latestMode = useMemo<"quick" | "full" | null>(() => {
    if (!desktop.quick && !desktop.full) return null;
    const quickAt = desktop.quick ? new Date(desktop.quick.completedAt).getTime() : 0;
    const fullAt = desktop.full ? new Date(desktop.full.completedAt).getTime() : 0;
    return fullAt >= quickAt ? "full" : "quick";
  }, [desktop.quick, desktop.full]);

  const page = (() => {
    switch (active) {
      case "processes":
        return <ProcessesPage full={desktop.full} busy={desktop.busy} onScan={() => void desktop.refresh("full")} />;
      case "storage":
        return <StoragePage quick={desktop.quick} full={desktop.full} />;
      case "developer":
        return <DeveloperPage full={desktop.full} busy={desktop.busy} onScan={() => void desktop.refresh("full")} />;
      case "settings":
        return <SettingsPage settings={settings} onChange={setSettings} appInfo={desktop.appInfo} />;
      case "overview":
      default:
        return (
          <OverviewPage
            quick={desktop.quick}
            full={desktop.full}
            history={desktop.history}
            busy={desktop.busy}
            onRefresh={() => void desktop.refresh("quick")}
            onNavigate={navigate}
          />
        );
    }
  })();

  const meta = PAGE_META[active];

  return (
    <>
      <a className="skip-link" href="#main-content" onClick={(event) => {
        event.preventDefault();
        document.getElementById("main-content")?.focus();
      }}>Skip to main content</a>
      <div className="app-shell">
        <Sidebar
          active={active}
          showDeveloperTools={settings.showDeveloperTools}
          version={desktop.appInfo?.version || "0.2.0"}
          onNavigate={navigate}
        />
        <div className="main">
          <Topbar
            eyebrow={meta.eyebrow}
            title={meta.title}
            subtitle={meta.subtitle}
            busy={desktop.busy}
            mode={latestMode}
            lastCompleted={lastCompleted}
            theme={settings.theme}
            onRefresh={() => void refreshCurrent()}
            onCancel={() => void desktop.cancel()}
            onToggleTheme={() =>
              setSettings((current) => ({ ...current, theme: current.theme === "dark" ? "light" : "dark" }))
            }
          />
          <main id="main-content" className="content scrollbar-thin" tabIndex={0} aria-labelledby="page-title">
            <div className="content-inner grid" style={{ gap: 16 }}>
              {desktop.error ? (
                <Notice tone="error">
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
                    <span><AlertTriangle size={14} aria-hidden="true" /> {desktop.error}</span>
                    <Button
                      variant="ghost"
                      onClick={() => void desktop.refresh(active === "developer" || active === "processes" ? "full" : "quick")}
                    >
                      <RefreshCw size={13} /> Try again
                    </Button>
                  </div>
                </Notice>
              ) : null}
              {desktop.notice ? <Notice tone="warn">{desktop.notice}</Notice> : null}
              {page}
            </div>
          </main>
        </div>
      </div>
    </>
  );
}
