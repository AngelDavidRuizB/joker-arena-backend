export function Background() {
  return (
    <div aria-hidden="true" className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
      <div className="absolute inset-x-0 bottom-0 h-[70vh] bg-grid animate-grid-move opacity-70 [transform:perspective(600px)_rotateX(58deg)] [transform-origin:bottom]" />
      <div className="absolute -left-32 top-10 h-96 w-96 rounded-full bg-neon-violet/25 blur-[110px] animate-pulse-glow" />
      <div className="absolute -right-24 top-1/3 h-96 w-96 rounded-full bg-neon-magenta/20 blur-[120px] animate-pulse-glow [animation-delay:1.2s]" />
      <div className="absolute bottom-0 left-1/3 h-80 w-80 rounded-full bg-neon-cyan/15 blur-[120px] animate-pulse-glow [animation-delay:2s]" />
    </div>
  );
}
