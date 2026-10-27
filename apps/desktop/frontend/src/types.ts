export type NavKey = "overview" | "processes" | "storage" | "developer" | "settings";
export type Theme = "dark" | "light";
export type ScanMode = "quick" | "full";

export interface Settings {
  theme: Theme;
  autoRefreshSeconds: number;
  showDeveloperTools: boolean;
}

export interface Sample {
  at: string;
  memoryPercent: number;
  cpuPercent: number;
}

export const DEFAULT_SETTINGS: Settings = {
  theme: "dark",
  autoRefreshSeconds: 0,
  showDeveloperTools: true,
};
