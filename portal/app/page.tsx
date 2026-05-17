import Link from "next/link";

export const dynamic = "force-dynamic";

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

function ScoreBadge({ score }: { score: number | null }) {
  if (score === null) {
    return (
      <span className="px-2 py-0.5 rounded text-xs font-mono bg-gray-100 text-gray-500 whitespace-nowrap">
        no score
      </span>
    );
  }
  const color =
    score >= 80
      ? "bg-green-100 text-green-800"
      : score >= 60
      ? "bg-amber-100 text-amber-800"
      : "bg-red-100 text-red-800";
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-mono whitespace-nowrap ${color}`}>
      {score}/100
    </span>
  );
}

async function getChanges(): Promise<Change[]> {
  const apiUrl = process.env.API_URL ?? "http://localhost:8000";
  try {
    const res = await fetch(`${apiUrl}/api/v1/changes`, { cache: "no-store" });
    if (!res.ok) return [];
    return res.json();
  } catch {
    return [];
  }
}

export default async function Home() {
  const changes = await getChanges();

  return (
    <main className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Pending Changes</h1>

        {changes.length === 0 ? (
          <div className="rounded border border-dashed border-gray-300 bg-white p-12 text-center text-gray-400 text-sm">
            No pending changes.
          </div>
        ) : (
          <ul className="space-y-4">
            {changes.map((c) => {
              const resources = parseResourceCounts(c.TerraformPlanOutput);
              return (
                <li key={c.ID}>
                  <Link
                    href={`/changes/${c.ID}`}
                    className="block rounded border border-gray-200 bg-white p-5 hover:border-gray-400 transition-colors"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <p className="text-sm text-gray-800 leading-relaxed">{c.NaturalLanguageReq}</p>
                      <ScoreBadge score={c.CheckovScore} />
                    </div>

                    <div className="mt-3 flex items-center gap-4 text-xs font-mono text-gray-500">
                      {resources ? (
                        <>
                          <span className="text-green-700">+{resources.add} add</span>
                          <span className="text-amber-700">~{resources.change} change</span>
                          <span className="text-red-700">−{resources.destroy} destroy</span>
                        </>
                      ) : (
                        <span>resources unknown</span>
                      )}
                      {c.CorrectionAttempts > 0 && (
                        <span className="ml-auto text-gray-400">
                          {c.CorrectionAttempts} correction
                          {c.CorrectionAttempts !== 1 ? "s" : ""}
                        </span>
                      )}
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </main>
  );
}
