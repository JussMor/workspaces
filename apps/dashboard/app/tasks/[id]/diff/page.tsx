interface TaskDiffPageProps {
  params: { id: string };
}

export default function TaskDiffPage({ params }: TaskDiffPageProps) {
  return (
    <main className="min-h-screen bg-forge-bg px-8 py-12">
      <h1 className="text-2xl font-bold text-white font-mono mb-2">
        Diff — Task {params.id}
      </h1>
      <p className="text-slate-500 font-mono text-sm">
        — Coming in Week 10 (Layer 06 — Orchestrator API)
      </p>
    </main>
  );
}
