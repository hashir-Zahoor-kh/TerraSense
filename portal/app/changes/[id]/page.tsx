"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

interface CheckovWarning {
  check_id: string;
  check_type: string;
  resource: string;
  message: string;
  severity: string;
}

interface Change {
  ID: string;
  NaturalLanguageReq: string;
  GeneratedHCL: string | null;
  TerraformPlanOutput: string | null;
  CheckovScore: number | null;
  CheckovWarnings: CheckovWarning[] | null;
  CorrectionAttempts: number;
  Status: string;
  CreatedAt: string;
  UpdatedAt: string;
}

interface ResourceCounts {
  add: number;
  change: number;
  destroy: number;
}

function parseResourceCounts(planOutput: string | null): ResourceCounts | null {
  if (!planOutput) return null;
  const match = planOutput.match(
    /Plan:\s*(\d+)\s+to add,\s*(\d+)\s+to change,\s*(\d+)\s+to destroy/
  );
  if (!match) return null;
  return {
    add: parseInt(match[1], 10),
    change: parseInt(match[2], 10),
    destroy: parseInt(match[3], 10),
  };
}

export default function ChangePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();

  const [change, setChange] = useState<Change | null>(null);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [busy, setBusy] = useState(false);

  const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

  useEffect(() => {
    fetch(`${apiUrl}/api/v1/changes/${id}`)
      .then((r) => {
        if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
        return r.json() as Promise<Change>;
      })
      .then(setChange)
      .catch((e: Error) => setFetchError(e.message))
      .finally(() => setLoading(false));
  }, [id, apiUrl]);

  async function approve() {
    setBusy(true);
    try {
      const r = await fetch(`${apiUrl}/api/v1/changes/${id}/approve`, {
        method: "POST",
      });
      if (!r.ok) throw new Error(await r.text());
      router.push("/");
      router.refresh();
    } catch (e) {
      alert(`Approve failed: ${e instanceof Error ? e.message : e}`);
    } finally {
      setBusy(false);
    }
  }

  async function reject() {
    if (!rejectReason.trim()) return;
    setBusy(true);
    try {
      const r = await fetch(`${apiUrl}/api/v1/changes/${id}/reject`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason: rejectReason }),
      });
      if (!r.ok) throw new Error(await r.text());
      router.push("/");
      router.refresh();
    } catch (e) {
      alert(`Reject failed: ${e instanceof Error ? e.message : e}`);
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <main className="p-8 text-gray-500 text-sm">Loading...</main>
    );
  }

  if (fetchError) {
    return (
      <main className="p-8 text-red-600 text-sm">Error: {fetchError}</main>
    );
  }

  if (!change) return null;

  const resources = parseResourceCounts(change.TerraformPlanOutput);

  return (
    <main className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-3xl mx-auto space-y-6">

        {/* Header */}
        <div>
          <a href="/" className="text-sm text-gray-500 hover:text-gray-800">
            ← Back
          </a>
          <h1 className="text-xl font-semibold text-gray-900 mt-2">
            {change.NaturalLanguageReq}
          </h1>
        </div>

        {/* Plan summary */}
        {resources && (
          <div className="flex gap-4 text-sm font-mono">
            <span className="text-green-700">+{resources.add} add</span>
            <span className="text-amber-700">~{resources.change} change</span>
            <span className="text-red-700">−{resources.destroy} destroy</span>
          </div>
        )}

        {/* Generated HCL */}
        {change.GeneratedHCL ? (
          <div>
            <h2 className="text-sm font-medium text-gray-700 mb-2">Generated HCL</h2>
            <pre className="rounded border border-gray-200 bg-gray-900 text-gray-100 text-xs font-mono p-4 overflow-x-auto whitespace-pre">
              {change.GeneratedHCL}
            </pre>
          </div>
        ) : (
          <p className="text-sm text-gray-400">No HCL generated yet.</p>
        )}

        {/* Checkov warnings */}
        {change.CheckovWarnings && change.CheckovWarnings.length > 0 && (
          <div>
            <h2 className="text-sm font-medium text-gray-700 mb-2">
              Checkov Warnings ({change.CheckovWarnings.length})
            </h2>
            <ul className="space-y-2">
              {change.CheckovWarnings.map((w, i) => (
                <li
                  key={i}
                  className="rounded border border-gray-200 bg-white p-3 text-xs font-mono"
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-semibold text-red-700">{w.check_id}</span>
                    <span className="text-gray-400 uppercase">{w.severity}</span>
                  </div>
                  <div className="text-gray-800">{w.resource}</div>
                  <div className="text-gray-500 mt-0.5">{w.message}</div>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Actions */}
        {change.Status === "pending" ? (
          <div>
            {!rejecting ? (
              <div className="flex gap-3">
                <button
                  onClick={approve}
                  disabled={busy}
                  className="px-4 py-2 rounded bg-green-600 text-white text-sm font-medium hover:bg-green-700 disabled:opacity-50"
                >
                  Approve
                </button>
                <button
                  onClick={() => setRejecting(true)}
                  disabled={busy}
                  className="px-4 py-2 rounded bg-red-600 text-white text-sm font-medium hover:bg-red-700 disabled:opacity-50"
                >
                  Reject
                </button>
              </div>
            ) : (
              <div className="space-y-2">
                <textarea
                  value={rejectReason}
                  onChange={(e) => setRejectReason(e.target.value)}
                  placeholder="Reason for rejection..."
                  rows={3}
                  className="w-full rounded border border-gray-300 p-2 text-sm font-mono resize-none focus:outline-none focus:border-gray-500"
                />
                <div className="flex gap-3">
                  <button
                    onClick={reject}
                    disabled={busy || !rejectReason.trim()}
                    className="px-4 py-2 rounded bg-red-600 text-white text-sm font-medium hover:bg-red-700 disabled:opacity-50"
                  >
                    Confirm Reject
                  </button>
                  <button
                    onClick={() => {
                      setRejecting(false);
                      setRejectReason("");
                    }}
                    disabled={busy}
                    className="px-4 py-2 rounded border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 disabled:opacity-50"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            )}
          </div>
        ) : (
          <p className="text-sm text-gray-500">
            Status:{" "}
            <span className="font-mono">{change.Status}</span>
          </p>
        )}
      </div>
    </main>
  );
}
