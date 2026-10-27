import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DesktopService } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop";
import type { AppInfo, FullResult, QuickResult } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import type { Sample, ScanMode } from "../types";

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "The desktop bridge returned an unexpected error.";
}

export interface DesktopState {
  appInfo: AppInfo | null;
  quick: QuickResult | null;
  full: FullResult | null;
  overview: QuickResult["overview"] | null;
  history: Sample[];
  busy: ScanMode | null;
  error: string | null;
  notice: string | null;
  refresh: (mode: ScanMode) => Promise<void>;
  cancel: () => Promise<void>;
}

export function useDesktop(autoRefreshSeconds: number): DesktopState {
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null);
  const [quick, setQuick] = useState<QuickResult | null>(null);
  const [full, setFull] = useState<FullResult | null>(null);
  const [history, setHistory] = useState<Sample[]>([]);
  const [busy, setBusy] = useState<ScanMode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const mounted = useRef(true);
  const busyRef = useRef(false);

  const addSample = useCallback((overview: QuickResult["overview"]) => {
    setHistory((current) => {
      const next = [
        ...current,
        {
          at: overview.capturedAt,
          memoryPercent: overview.memoryUsedPercent,
          cpuPercent: overview.cpuKnown ? overview.cpuPercent : 0,
        },
      ];
      return next.slice(-40);
    });
  }, []);

  const runScan = useCallback(
    async (mode: ScanMode): Promise<boolean> => {
      if (busyRef.current) return false;
      busyRef.current = true;
      setBusy(mode);
      setError(null);
      setNotice(null);
      try {
        const result = mode === "quick" ? await DesktopService.QuickScan() : await DesktopService.FullScan();
        if (!mounted.current) return false;
        if (result.status === "cancelled") {
          setNotice("Scan cancelled. Existing observations were kept.");
          return false;
        }
        if (mode === "quick") {
          setQuick(result as QuickResult);
        } else {
          setFull(result as FullResult);
        }
        addSample(result.overview);
        if (result.status === "timeout") {
          setNotice("The scan reached its time limit. Partial observations are shown in the page.");
        }
        return true;
      } catch (cause) {
        if (mounted.current) {
          setError(errorMessage(cause));
        }
        return false;
      } finally {
        busyRef.current = false;
        if (mounted.current) setBusy(null);
      }
    },
    [addSample],
  );

  const refresh = useCallback(
    async (mode: ScanMode) => {
      await runScan(mode);
    },
    [runScan],
  );

  const cancel = useCallback(async () => {
    try {
      const cancelled = await DesktopService.CancelScan();
      if (cancelled) setNotice("Cancelling the scan…");
    } catch (cause) {
      if (mounted.current) setError(errorMessage(cause));
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    let cancelled = false;
    (async () => {
      try {
        const info = await DesktopService.GetAppInfo();
        if (!cancelled && mounted.current) setAppInfo(info);
      } catch (cause) {
        if (!cancelled && mounted.current) setError(errorMessage(cause));
        return;
      }
      if (cancelled || !mounted.current) return;
      const quickOK = await runScan("quick");
      if (quickOK && !cancelled && mounted.current) {
        await runScan("full");
      }
    })();
    return () => {
      cancelled = true;
      mounted.current = false;
      if (busyRef.current) {
        void DesktopService.CancelScan();
      }
    };
  }, [runScan]);

  useEffect(() => {
    if (autoRefreshSeconds <= 0) return;
    const timer = window.setInterval(() => {
      if (!busyRef.current && mounted.current) {
        void runScan("quick");
      }
    }, autoRefreshSeconds * 1000);
    return () => window.clearInterval(timer);
  }, [autoRefreshSeconds, runScan]);

  const overview = useMemo(() => {
    const quickAt = quick ? new Date(quick.overview.capturedAt).getTime() : 0;
    const fullAt = full ? new Date(full.overview.capturedAt).getTime() : 0;
    if (quick && quickAt >= fullAt) return quick.overview;
    if (full) return full.overview;
    return null;
  }, [quick, full]);

  return { appInfo, quick, full, overview, history, busy, error, notice, refresh, cancel };
}
