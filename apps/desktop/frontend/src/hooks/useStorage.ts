import { useCallback, useEffect, useRef, useState } from "react";
import type {
  StorageChildren,
  StorageNode,
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
  busy: boolean;
  error: string | null;
  chooseRoot: () => Promise<void>;
  startScan: (options: StorageScanOptions) => Promise<void>;
  cancel: () => Promise<void>;
  openNode: (node: StorageNode) => Promise<void>;
  goToTrail: (index: number) => Promise<void>;
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
            setError(next.warnings?.[0] || "Storage scan failed.");
          } else {
            setChildren(next.nodes ?? []);
            setCurrent(null);
            setTrail([]);
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

  const openNode = useCallback(
    async (node: StorageNode) => {
      if (!view?.scanId || node.kind !== "dir") return;
      try {
        const result: StorageChildren = await StorageService.GetStorageNode(view.scanId, node.id);
        if (!mounted.current) return;
        setCurrent(result.parent);
        setChildren(result.children ?? []);
        setTrail((existing) => [...existing, result.parent]);
      } catch (cause) {
        if (mounted.current) setError(errorMessage(cause));
      }
    },
    [view?.scanId],
  );

  const goToTrail = useCallback(
    async (index: number) => {
      if (!view?.scanId) return;
      if (index < 0) {
        setCurrent(null);
        setTrail([]);
        setChildren(view.nodes ?? []);
        return;
      }
      const target = trail[index];
      if (!target) return;
      try {
        const result = await StorageService.GetStorageNode(view.scanId, target.id);
        if (!mounted.current) return;
        setCurrent(result.parent);
        setChildren(result.children ?? []);
        setTrail(trail.slice(0, index + 1));
      } catch (cause) {
        if (mounted.current) setError(errorMessage(cause));
      }
    },
    [view?.scanId, view?.nodes, trail],
  );

  return { root, view, current, children, trail, busy, error, chooseRoot, startScan, cancel, openNode, goToTrail };
}
