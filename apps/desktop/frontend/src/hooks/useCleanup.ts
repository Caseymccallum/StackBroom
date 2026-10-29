import { useCallback, useEffect, useRef, useState } from "react";
import type {
  CleanupDiscoveryView,
  CleanupExecutionView,
  CleanupPreviewView,
  CleanupRule,
} from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import * as CleanupService from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/cleanupservice";

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "The cleanup discovery service returned an unexpected error.";
}

export interface CleanupState {
  rules: CleanupRule[];
  rule: CleanupRule | null;
  discovery: CleanupDiscoveryView | null;
  preview: CleanupPreviewView | null;
  busy: boolean;
  error: string | null;
  loadRules: () => Promise<void>;
  start: (rule: CleanupRule) => Promise<void>;
  cancel: () => Promise<void>;
  createPreview: (candidateIDs: string[]) => Promise<void>;
  confirmPermanent: (previewID: string) => Promise<CleanupExecutionView | null>;
  clearPreview: () => void;
}

export function useCleanup(): CleanupState {
  const [rules, setRules] = useState<CleanupRule[]>([]);
  const [rule, setRule] = useState<CleanupRule | null>(null);
  const [discovery, setDiscovery] = useState<CleanupDiscoveryView | null>(null);
  const [preview, setPreview] = useState<CleanupPreviewView | null>(null);
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

  const loadRules = useCallback(async () => {
    try {
      const catalogue = await CleanupService.ListCleanupRules();
      if (mounted.current) setRules(catalogue ?? []);
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    }
  }, []);

  const poll = useCallback(async (discoveryID: string) => {
    const token = ++pollToken.current;
    while (mounted.current && token === pollToken.current) {
      try {
        const next = await CleanupService.GetCleanupDiscovery(discoveryID);
        if (!mounted.current || token !== pollToken.current) return;
        setDiscovery(next);
        if (next.status !== "running") {
          setBusy(false);
          if (next.status === "error") setError(next.errorMessage || "Cleanup discovery failed.");
          return;
        }
      } catch (cause) {
        if (mounted.current && token === pollToken.current) {
          setError(errorMessage(cause));
          setBusy(false);
        }
        return;
      }
      await new Promise((resolve) => window.setTimeout(resolve, 300));
    }
  }, []);

  const start = useCallback(async (selected: CleanupRule) => {
    if (busy) return;
    setBusy(true);
    setError(null);
    setRule(selected);
    setDiscovery(null);
    setPreview(null);
    try {
      const discoveryID = await CleanupService.StartCleanupDiscovery(selected.id, selected.id === "crash-dumps-old" ? 200 : 500);
      if (mounted.current) void poll(discoveryID);
    } catch (cause) {
      if (mounted.current) {
        setError(errorMessage(cause));
        setBusy(false);
      }
    }
  }, [busy, poll]);

  const cancel = useCallback(async () => {
    if (!discovery?.discoveryId) return;
    try {
      const requested = await CleanupService.CancelCleanupDiscovery(discovery.discoveryId);
      if (!requested) setError("The discovery was already finished.");
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    }
  }, [discovery?.discoveryId]);

  const createPreview = useCallback(async (candidateIDs: string[]) => {
    if (!discovery?.discoveryId) return;
    setBusy(true);
    setError(null);
    try {
      const next = await CleanupService.CreateCleanupPreview(discovery.discoveryId, candidateIDs);
      if (mounted.current) setPreview(next);
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }, [discovery?.discoveryId]);

  const confirmPermanent = useCallback(async (previewID: string) => {
    if (!previewID) return null;
    setBusy(true);
    setError(null);
    try {
      const next = await CleanupService.ConfirmPermanentCleanup(previewID);
      return next ?? null;
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
      return null;
    } finally {
      if (mounted.current) setBusy(false);
    }
  }, []);

  const clearPreview = useCallback(() => setPreview(null), []);

  return { rules, rule, discovery, preview, busy, error, loadRules, start, cancel, createPreview, confirmPermanent, clearPreview };
}
