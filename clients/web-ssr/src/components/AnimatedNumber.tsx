import { animate, motion, useMotionValue, useReducedMotion, useTransform } from 'framer-motion';
import { useEffect } from 'react';

interface AnimatedNumberProps {
  value: number;
  className?: string;
  duration?: number;
}

/** Counter that tweens smoothly between values (instant with prefers-reduced-motion). */
export function AnimatedNumber({ value, className, duration = 0.45 }: AnimatedNumberProps) {
  const reduce = useReducedMotion();
  const mv = useMotionValue(value);
  const text = useTransform(mv, (v) => Math.round(v).toLocaleString('es-CO'));

  useEffect(() => {
    if (reduce) {
      mv.set(value);
      return;
    }
    const controls = animate(mv, value, { duration, ease: 'easeOut' });
    return () => controls.stop();
  }, [value, reduce, duration, mv]);

  return <motion.span className={className}>{text}</motion.span>;
}
