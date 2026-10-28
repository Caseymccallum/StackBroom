export type NavKey = "overview" | "processes" | "storage" | "cleaning" | "developer" | "settings";
export type Theme = "dark" | "light";
export type ScanMode = "quick" | "full";

export interface Settings {
  theme: Theme;
  autoRefreshSeconds: number;
  showDeveloperTools: boolean;
}

export interface Sample {
  at: string;
  memoryPercent: number | null;
  cpuPercent: number | null;
}

export const DEFAULT_SETTINGS: Settings = {
  theme: "dark",
  autoRefreshSeconds: 0,
  showDeveloperTools: true,
};
