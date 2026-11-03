import { motion } from 'framer-motion';
import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';

type Variant = 'cyan' | 'magenta' | 'amber' | 'ghost';

interface NeonButtonProps
  extends Omit<
    ButtonHTMLAttributes<HTMLButtonElement>,
    'onDrag' | 'onDragStart' | 'onDragEnd' | 'onAnimationStart' | 'onAnimationEnd'
  > {
  variant?: Variant;
  loading?: boolean;
  icon?: ReactNode;
  block?: boolean;
}

const STYLES: Record<Variant, string> = {
  cyan: 'border-neon-cyan/70 bg-neon-cyan/10 text-neon-cyan hover:bg-neon-cyan/20 hover:shadow-neon-cyan',
  magenta:
    'border-neon-magenta/70 bg-neon-magenta/10 text-neon-magenta hover:bg-neon-magenta/20 hover:shadow-neon-magenta',
  amber: 'border-neon-amber/80 bg-neon-amber/15 text-neon-amber hover:bg-neon-amber/25 hover:shadow-neon-amber',
  ghost: 'border-white/15 bg-white/5 text-slate-200 hover:bg-white/10',
};

export const NeonButton = forwardRef<HTMLButtonElement, NeonButtonProps>(function NeonButton(
  { variant = 'cyan', loading = false, icon, block = false, disabled, children, className = '', ...rest },
  ref,
) {
  const isDisabled = disabled || loading;
  return (
    <motion.button
      ref={ref}
      whileTap={isDisabled ? undefined : { scale: 0.96 }}
      whileHover={isDisabled ? undefined : { y: -1 }}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      className={`inline-flex items-center justify-center gap-2 rounded-xl border px-5 py-3 font-display text-xs font-bold uppercase tracking-[0.16em] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon-cyan focus-visible:ring-offset-2 focus-visible:ring-offset-ink-950 disabled:cursor-not-allowed disabled:opacity-40 ${
        STYLES[variant]
      } ${block ? 'w-full' : ''} ${className}`}
      {...rest}
    >
      {loading ? (
        <span
          className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"
          aria-hidden="true"
        />
      ) : (
        icon
      )}
      {children}
    </motion.button>
  );
});
