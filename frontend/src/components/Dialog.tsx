import { Dialog as DialogRoot, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  title: string
  description?: string
  children: React.ReactNode
  footer?: React.ReactNode
  width?: string
}

export default function Dialog({ open, onOpenChange, title, description, children, footer, width }: Props) {
  return (
    <DialogRoot open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn(width ?? 'max-w-lg')}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>

        <div className="text-sm">{children}</div>

        {footer && (
          <div className="flex flex-col-reverse gap-2 [&>button]:w-full sm:flex-row-reverse sm:gap-3 sm:[&>button]:w-auto">
            {footer}
          </div>
        )}
      </DialogContent>
    </DialogRoot>
  )
}
