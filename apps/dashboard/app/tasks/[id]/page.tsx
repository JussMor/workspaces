interface TaskDetailPageProps {
  params: { id: string };
}

export default function TaskDetailPage({ params }: TaskDetailPageProps) {
  return (
    <main className="min-h-screen bg-forge-bg px-8 py-12">
      <h1 className="text-2xl font-bold text-white font-mono mb-2">
        Task: {params.id}
      </h1>
      <p className="text-slate-500 font-mono text-sm">
        — Coming in Week 7-8 (Layer 04 — Coordinator)
      </p>
    </main>
  );
}
