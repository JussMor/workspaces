const LAYERS = [
  { num: "01", name: "Sandbox", week: "1-2", color: "#00FFB2" },
  { num: "02", name: "Agent Engine", week: "3-4", color: "#FF6B35" },
  { num: "03", name: "Context Engine", week: "5-6", color: "#A78BFA" },
  { num: "04", name: "Coordinator", week: "7-8", color: "#F59E0B" },
  { num: "05", name: "GitHub", week: "9", color: "#34D399" },
  { num: "06", name: "Orchestrator API", week: "10", color: "#60A5FA" },
  { num: "07", name: "Dashboard", week: "11-12", color: "#F472B6" },
];

export default function Home() {
  return (
    <main className="min-h-screen bg-forge-bg flex flex-col items-center justify-center px-6 py-16">
      {/* Hero */}
      <div className="mb-16 text-center">
        <h1
          className="text-7xl font-bold tracking-widest font-mono mb-4"
          style={{
            color: "#00FFB2",
            textShadow: "0 0 40px #00FFB280, 0 0 80px #00FFB240",
          }}
        >
          FORGE
        </h1>
        <p className="text-slate-400 text-sm tracking-[0.3em] uppercase font-mono">
          AI Software Engineering Platform
        </p>
      </div>

      {/* Layer grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 w-full max-w-6xl">
        {LAYERS.map((layer) => (
          <div
            key={layer.num}
            className="bg-forge-panel border border-forge-border rounded-lg p-5 flex flex-col gap-3"
          >
            {/* Color bar */}
            <div
              className="h-1 rounded-full w-full"
              style={{ backgroundColor: layer.color }}
            />
            <div className="flex items-start justify-between">
              <div>
                <p
                  className="text-xs font-mono font-bold mb-1"
                  style={{ color: layer.color }}
                >
                  LAYER {layer.num}
                </p>
                <h2 className="text-white font-mono font-semibold text-sm">
                  {layer.name}
                </h2>
              </div>
              <span className="text-xs font-mono text-slate-500 bg-slate-800 px-2 py-1 rounded">
                W{layer.week}
              </span>
            </div>
            <p className="text-xs text-slate-500 font-mono">
              Coming Week {layer.week}
            </p>
          </div>
        ))}
      </div>
    </main>
  );
}
