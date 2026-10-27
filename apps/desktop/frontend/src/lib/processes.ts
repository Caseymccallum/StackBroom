import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";

export type ProcessSort = "memory" | "name" | "pid" | "started" | "category";

export interface ProcessFilter {
  query: string;
  category: string;
  sort: ProcessSort;
}

export function filterAndSortProcesses(processes: ProcessRow[], filter: ProcessFilter): ProcessRow[] {
  const query = filter.query.trim().toLowerCase();
  const filtered = processes.filter((process) => {
    if (filter.category !== "all" && process.category !== filter.category) return false;
    if (!query) return true;
    const haystack = [
      process.name,
      process.projectName ?? "",
      String(process.pid),
      ...(process.requestedPorts ?? []).map(String),
      ...(process.listeningPorts ?? []).map(String),
    ]
      .join(" ")
      .toLowerCase();
    return haystack.includes(query);
  });

  return filtered.sort((a, b) => {
    switch (filter.sort) {
      case "name":
        return a.name.localeCompare(b.name);
      case "pid":
        return a.pid - b.pid;
      case "started":
        return (a.startedAt ?? "").localeCompare(b.startedAt ?? "");
      case "category":
        return a.category.localeCompare(b.category) || a.name.localeCompare(b.name);
      case "memory":
      default:
        if (a.memoryKnown !== b.memoryKnown) return a.memoryKnown ? -1 : 1;
        if (a.memoryBytes !== b.memoryBytes) return b.memoryBytes - a.memoryBytes;
        return a.name.localeCompare(b.name);
    }
  });
}
