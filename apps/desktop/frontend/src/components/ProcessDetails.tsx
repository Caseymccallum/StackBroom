import { useEffect, useRef, useState } from "react";
import { X, AlertTriangle, Shield, Power, Trash2, Gauge } from "lucide-react";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import * as ProcessControlService from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/processcontrolservice";
import { Badge, Button } from "./ui";

interface ProcessDetailsProps {
  process: ProcessRow;
  onClose: () => void;
  onCompleted?: () => void;
  initialConfirm?: boolean;
}

interface PendingAction {
  action: string;
  priority?: number;
  label: string;
}

export function ProcessDetails({ process, onClose, onCompleted, initialConfirm }: ProcessDetailsProps) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; outcome: string; details: string; previous?: string; next?: string } | null>(null);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const confirmRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    closeRef.current?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab" || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previous?.focus?.();
    };
  }, [onClose]);

  useEffect(() => {
    if (initialConfirm) {
      setPending({ action: "terminate", label: "End Process" });
    }
  }, [initialConfirm]);

  useEffect(() => {
    if (pending) {
      confirmRef.current?.scrollIntoView?.({ block: "center" });
    }
  }, [pending]);

  const requestAction = (action: string, priority?: number, label?: string) => {
    if (loading) return;
    setResult(null);
    setPending({ action, priority, label: label || action.replace("_", " ") });
  };

  const executePending = async () => {
    if (!pending || loading) return;
    if (!process.targetToken) {
      setResult({ success: false, outcome: "refused", details: "This process has no verified target token. Refresh the process list." });
      setPending(null);
      return;
    }
    setLoading(true);
    setPending(null);
    try {
      await ProcessControlService.GetProcessDetails(process.pid, process.targetToken);
      const preview = await ProcessControlService.PreviewProcessAction(
        process.pid,
        pending.action,
        pending.priority || 0,
        process.targetToken,
      );
      if (!preview || !preview.operationId) throw new Error("The backend did not issue an operation preview.");
      const execution = await ProcessControlService.ConfirmProcessAction(preview.operationId);
      if (!execution) throw new Error("The backend returned no execution result.");
      setResult({
        success: execution.success === true,
        outcome: execution.outcome || "unknown",
        details: execution.details || "No detail supplied by the backend.",
        previous: execution.previousValue,
        next: execution.newValue,
      });
      if (execution.success) onCompleted?.();
    } catch (error) {
      setResult({
        success: false,
        outcome: "error",
        details: error instanceof Error ? error.message : "Unknown error occurred",
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" role="presentation">
      <div
        ref={dialogRef}
        className="rounded-lg max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        style={{ background: "var(--surface)", border: "1px solid var(--border-strong)", boxShadow: "var(--shadow)" }}
        role="dialog"
        aria-modal="true"
        aria-labelledby="process-details-title"
        aria-describedby="process-details-description"
      >
        <div className="p-6">
          <div className="flex items-start justify-between mb-4">
            <div>
              <h2 id="process-details-title" className="text-xl font-semibold">{process.name}</h2>
              <p id="process-details-description" className="text-sm text-muted">PID: {process.pid}. Review actions carefully; process termination cannot be undone.</p>
            </div>
            <button ref={closeRef} onClick={onClose} className="p-2 hover:bg-surface-2 rounded" aria-label="Close process details">
              <X size={20} />
            </button>
          </div>

          <div className="space-y-4">
            <div>
              <h3 className="text-sm font-semibold mb-2">Process Identity</h3>
              <div className="bg-surface-2 rounded p-3 space-y-1">
                <div className="flex justify-between">
                  <span className="text-muted">Name:</span>
                  <span className="font-mono">{process.name}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted">PID:</span>
                  <span className="font-mono">{process.pid}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted">Category:</span>
                  <Badge>{process.category}</Badge>
                </div>
              </div>
            </div>

            {!pending && (
            <div>
              <h3 className="text-sm font-semibold mb-2">Available Actions</h3>
              <div className="space-y-2">
                <Button onClick={() => requestAction("graceful_close", undefined, "Close Gracefully")} disabled={loading} className="w-full justify-start">
                  <Power size={16} className="mr-2" />
                  Close Gracefully
                  <span className="ml-auto text-xs text-muted">Sends WM_CLOSE</span>
                </Button>

                <Button onClick={() => requestAction("terminate", undefined, "End Process")} disabled={loading} className="w-full justify-start" style={{ backgroundColor: "var(--red)", color: "white" }}>
                  <Trash2 size={16} className="mr-2" />
                  End Process
                  <span className="ml-auto text-xs opacity-80">Force termination</span>
                </Button>

                <div className="pt-2 border-t border-border">
                  <p className="text-xs text-muted mb-2">Priority Class:</p>
                  <div className="grid grid-cols-3 gap-2">
                    {["Idle", "Below Normal", "Normal", "Above Normal", "High"].map((p) => (
                      <Button key={p} onClick={() => requestAction("set_priority", priorityValue(p), `Set priority to ${p}`)} disabled={loading} variant="ghost">
                        <Gauge size={12} className="mr-1" />
                        {p}
                      </Button>
                    ))}
                  </div>
                </div>
              </div>
            </div>
            )}

            {pending && (
              <div ref={confirmRef} className="rounded-lg border border-[color:var(--amber)] bg-[color:var(--amber-soft)] p-3" role="alertdialog" aria-labelledby="confirm-title" aria-describedby="confirm-description">
                <p id="confirm-title" className="font-semibold text-[color:var(--amber)]">Confirm {pending.label}</p>
                <p id="confirm-description" className="mt-1 text-sm" style={{ color: "var(--text-soft)" }}>
                  Target: {process.name} (PID {process.pid}). {pending.action === "terminate"
                    ? "Unsaved application data will be lost and this cannot be undone."
                    : pending.action === "graceful_close"
                      ? "The application may prompt to save work. If it ignores the request, StackBroom will not force it closed."
                      : "This changes only the scheduling priority, not a CPU limit."}
                </p>
                <div className="mt-3 flex gap-2">
                  <Button autoFocus onClick={executePending} disabled={loading} style={{ backgroundColor: "var(--red)", color: "white" }}>
                    Confirm {pending.label}
                  </Button>
                  <Button onClick={() => setPending(null)} disabled={loading} variant="ghost">Cancel</Button>
                </div>
              </div>
            )}

            {result && (
              <div className={`p-3 rounded ${result.success ? "bg-[color:var(--accent-soft)]" : "bg-[color:var(--red-soft)]"}`} role="status" aria-live="polite">
                <div className="flex items-start gap-2">
                  {result.success ? <Shield size={16} /> : <AlertTriangle size={16} />}
                  <div>
                    <p className="font-semibold">{result.outcome}</p>
                    <p className="text-sm">{result.details}</p>
                    {result.previous && result.next ? (
                      <p className="text-sm">Observed change: {result.previous} → {result.next}</p>
                    ) : null}
                  </div>
                </div>
              </div>
            )}

            <div className="rounded p-3" style={{ background: "var(--amber-soft)", border: "1px solid color-mix(in srgb, var(--amber) 30%, transparent)" }}>
              <div className="flex items-start gap-2">
                <AlertTriangle size={16} style={{ color: "var(--amber)" }} />
                <div className="text-sm">
                  <p className="font-semibold" style={{ color: "var(--amber)" }}>Irreversible action warning</p>
                  <p style={{ color: "var(--text-soft)" }}>
                    Forced termination loses unsaved work. A graceful close may be ignored; StackBroom never escalates automatically.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function priorityValue(name: string): number {
  switch (name) {
    case "Idle": return 0x00000040;
    case "Below Normal": return 0x00004000;
    case "Normal": return 0x00000020;
    case "Above Normal": return 0x00008000;
    case "High": return 0x00000080;
    default: return 0x00000020;
  }
}
