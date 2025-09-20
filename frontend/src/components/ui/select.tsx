import * as React from 'react';
import { cn } from '../../lib/utils';

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
}
export function Select({ className, label, children, ...props }: SelectProps) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      {label && <span className="text-muted-foreground">{label}</span>}
      <select className={cn('rounded-xl border bg-transparent px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring', className)} {...props}>
        {children}
      </select>
    </label>
  );
}
