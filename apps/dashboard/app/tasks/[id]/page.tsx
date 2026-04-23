interface TaskDetailPageProps {
  params: Promise<{ id: string }>;
}

export default async function TaskDetailPage({ params }: TaskDetailPageProps) {
  const { id } = await params;

  return (
    <main className="min-h-screen bg-forge-bg px-8 py-12">
      <h1 className="text-2xl font-bold text-white font-mono mb-2">
        Task: {id}
      </h1>
      <p className="text-slate-500 font-mono text-sm">
        — Coming in Week 7-8 (Layer 04 — Coordinator)
      </p>
    </main>
  );
}
