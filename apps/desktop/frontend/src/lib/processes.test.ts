import { describe, expect, it } from "vitest";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import { filterAndSortProcesses } from "./processes";

const processes: ProcessRow[] = [
  { pid: 1, parentPid: 0, name: "node.exe", runtime: "node", category: "dev-server", memoryBytes: 100, memoryKnown: true, startTimeKnown: true, projectName: "HerdHealth", requestedPorts: [3000] },
  { pid: 2, parentPid: 1, name: "Code.exe", runtime: "unknown", category: "other", memoryBytes: 500, memoryKnown: true, startTimeKnown: false },
  { pid: 3, parentPid: 0, name: "python.exe", runtime: "python", category: "dev-tool", memoryBytes: 0, memoryKnown: false, startTimeKnown: true },
];

describe("process filtering", () => {
  it("searches ports, pids, projects and names", () => {
    expect(filterAndSortProcesses(processes, { query: "3000", category: "all", sort: "name" }).map((p) => p.pid)).toEqual([1]);
    expect(filterAndSortProcesses(processes, { query: "herd", category: "all", sort: "name" }).map((p) => p.pid)).toEqual([1]);
    expect(filterAndSortProcesses(processes, { query: "python", category: "all", sort: "name" }).map((p) => p.pid)).toEqual([3]);
  });

  it("filters categories and sorts unknown memory last", () => {
    expect(filterAndSortProcesses(processes, { query: "", category: "dev-server", sort: "memory" })).toHaveLength(1);
    const sorted = filterAndSortProcesses(processes, { query: "", category: "all", sort: "memory" });
    expect(sorted.map((p) => p.pid)).toEqual([2, 1, 3]);
  });

  it("sorts names without mutating the source", () => {
    const sorted = filterAndSortProcesses(processes, { query: "", category: "all", sort: "name" });
    expect(sorted.map((p) => p.name)).toEqual(["Code.exe", "node.exe", "python.exe"]);
    expect(processes.map((p) => p.pid)).toEqual([1, 2, 3]);
  });
});
