import { useCallback, useEffect, useRef, useState } from "react";
import type {
  StorageNode,
  StorageNodePage,
  StorageRoot,
  StorageScanOptions,
  StorageScanView,
} from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import * as StorageService from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/storageservice";

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "The storage engine returned an unexpected error.";
}

export interface StorageState {
  root: StorageRoot | null;
  view: StorageScanView | null;
  current: StorageNode | null;
  children: StorageNode[];
  trail: StorageNode[];
  query: string;
  totalChildren: number;
  remainingLogical: number;
  nextCursor: string;
  busy: boolean;
  error: string | null;
  chooseRoot: () => Promise<void>;
  startScan: (options: StorageScanOptions) => Promise<void>;
  cancel: () => Promise<void>;
  openNode: (node: StorageNode) => Promise<void>;
  goToTrail: (index: number) => Promise<void>;
  search: (query: string) => Promise<void>;
  nextPage: () => Promise<void>;
}

const defaultOptions: StorageScanOptions = {
  includeHidden: false,
  includeSystem: false,
  maxDepth: 64,
  maxEntries: 250000,
};

export function useStorage(): StorageState {
  const [root, setRoot] = useState<StorageRoot | null>(null);
  const [view, setView] = useState<StorageScanView | null>(null);
  const [current, setCurrent] = useState<StorageNode | null>(null);
  const [children, setChildren] = useState<StorageNode[]>([]);
  const [trail, setTrail] = useState<StorageNode[]>([]);
  const [query, setQuery] = useState("");
  const [totalChildren, setTotalChildren] = useState(0);
  const [remainingLogical, setRemainingLogical] = useState(0);
  const [nextCursor, setNextCursor] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mounted = useRef(true);
  const pollToken = useRef(0);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      pollToken.current++;
    };
  }, []);

  const chooseRoot = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const selected = await StorageService.SelectStorageRoot();
      if (!mounted.current) return;
      setRoot(selected);
      setView(null);
      setCurrent(null);
      setChildren([]);
      setTrail([]);
      setQuery("");
      setTotalChildren(0);
      setRemainingLogical(0);
      setNextCursor("");
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }, []);

  const poll = useCallback(async (scanID: string) => {
    const token = ++pollToken.current;
    while (mounted.current && token === pollToken.current) {
      try {
        const next = await StorageService.GetStorageScan(scanID);
        if (!mounted.current || token !== pollToken.current) return;
        setView(next);
        if (next.status !== "running") {
          setBusy(false);
          if (next.status === "error") {
            setError(next.errorMessage || next.warnings?.[0] || "Storage scan failed.");
          } else {
            setChildren(next.nodes ?? []);
            setCurrent(null);
            setTrail([]);
            setQuery("");
            setTotalChildren(next.rootChildCount ?? (next.nodes ?? []).length);
            setRemainingLogical(next.rootRemaining ?? 0);
            setNextCursor(next.nodesNextCursor ?? "");
          }
          return;
        }
      } catch (cause) {
        if (mounted.current && token === pollToken.current) {
          setError(errorMessage(cause));
          setBusy(false);
        }
        return;
      }
      await new Promise((resolve) => window.setTimeout(resolve, 350));
    }
  }, []);

  const startScan = useCallback(
    async (options: StorageScanOptions = defaultOptions) => {
      if (!root || busy) return;
      setBusy(true);
      setError(null);
      setView(null);
      setCurrent(null);
      setChildren([]);
      setTrail([]);
      setQuery("");
      setTotalChildren(0);
      setRemainingLogical(0);
      setNextCursor("");
      try {
        const scanID = await StorageService.StartStorageScan(root.id, options);
        if (!mounted.current) return;
        void poll(scanID);
      } catch (cause) {
        if (mounted.current) {
          setError(errorMessage(cause));
          setBusy(false);
        }
      }
    },
    [root, busy, poll],
  );

  const cancel = useCallback(async () => {
    if (!view?.scanId) return;
    try {
      const requested = await StorageService.CancelStorageScan(view.scanId);
      if (!requested) setError("The scan was already finished.");
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    }
  }, [view?.scanId]);

  const loadPage = useCallback(
    async (nodeID: string, cursor: string, term: string, append: boolean): Promise<StorageNodePage | null> => {
      if (!view?.scanId) return null;
      try {
        const page = await StorageService.GetStorageNodePage(view.scanId, nodeID, cursor, 100, term);
        if (!mounted.current) return null;
        setCurrent(nodeID === view.rootNodeId ? null : page.node);
        setChildren((existing) => append ? [...existing, ...(page.children ?? [])] : (page.children ?? []));
        setTotalChildren(page.totalChildren);
        setRemainingLogical(page.remainingLogical);
        setNextCursor(page.nextCursor ?? "");
        setQuery(term);
        return page;
      } catch (cause) {
        if (mounted.current) setError(errorMessage(cause));
        return null;
      }
    },
    [view?.scanId, view?.rootNodeId],
  );

  const openNode = useCallback(
    async (node: StorageNode) => {
      if (!view?.scanId || node.kind !== "dir") return;
      const page = await loadPage(node.id, "", "", false);
      if (page) setTrail((existing) => [...existing, page.node]);
    },
    [view?.scanId, loadPage],
  );

  const goToTrail = useCallback(
    async (index: number) => {
      if (!view?.scanId) return;
      if (index < 0) {
        await loadPage(view.rootNodeId, "", "", false);
        setTrail([]);
        return;
      }
      const target = trail[index];
      if (!target) return;
      const page = await loadPage(target.id, "", "", false);
      if (page) setTrail(trail.slice(0, index + 1));
    },
    [view?.scanId, view?.rootNodeId, trail, loadPage],
  );

  const search = useCallback(
    async (term: string) => {
      if (!view?.scanId) return;
      await loadPage(current?.id ?? view.rootNodeId, "", term, false);
    },
    [view?.scanId, view?.rootNodeId, current?.id, loadPage],
  );

  const nextPage = useCallback(async () => {
    if (!view?.scanId || !nextCursor) return;
    await loadPage(current?.id ?? view.rootNodeId, nextCursor, query, true);
  }, [view?.scanId, view?.rootNodeId, current?.id, nextCursor, query, loadPage]);

  return { root, view, current, children, trail, query, totalChildren, remainingLogical, nextCursor, busy, error, chooseRoot, startScan, cancel, openNode, goToTrail, search, nextPage };
}
