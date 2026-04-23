interface TaskDiffPageProps {
  params: Promise<{ id: string }>;
}

export default async function TaskDiffPage({ params }: TaskDiffPageProps) {
  const { id } = await params;

  return (
    <main className="min-h-screen bg-forge-bg px-8 py-12">
      <h1 className="text-2xl font-bold text-white font-mono mb-2">
        Diff — Task {id}
      </h1>
      <p className="text-slate-500 font-mono text-sm">
        — Coming in Week 10 (Layer 06 — Orchestrator API)
      </p>
    </main>
  );
}
