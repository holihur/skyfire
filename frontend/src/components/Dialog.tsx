import * as DialogPrim from '@radix-ui/react-dialog'
import { Cross2Icon } from '@radix-ui/react-icons'
import { clsx } from 'clsx'

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
    <DialogPrim.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrim.Portal>
        <DialogPrim.Overlay className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=open]:fade-in" />
        <DialogPrim.Content
          className={clsx(
            'fixed left-1/2 top-1/2 z-50 w-[calc(100vw-2rem)] max-h-[90vh] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border border-[#263450] bg-[#0d1830] p-6 shadow-2xl shadow-black/50 focus:outline-none',
            width ?? 'max-w-lg',
          )}
        >
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <DialogPrim.Title className="text-lg font-semibold text-white">
                {title}
              </DialogPrim.Title>
              {description && (
                <DialogPrim.Description className="mt-1 text-sm text-slate-400">
                  {description}
                </DialogPrim.Description>
              )}
            </div>
            <DialogPrim.Close className="rounded-md p-1 text-slate-400 hover:bg-white/5 hover:text-white">
              <Cross2Icon className="h-4 w-4" />
            </DialogPrim.Close>
          </div>

          <div className="text-sm text-slate-200">{children}</div>

          {footer && <div className="mt-6 flex flex-row-reverse gap-3">{footer}</div>}
        </DialogPrim.Content>
      </DialogPrim.Portal>
    </DialogPrim.Root>
  )
}