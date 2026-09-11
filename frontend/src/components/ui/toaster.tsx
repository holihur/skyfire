import { CircleCheck, Info, TriangleAlert } from 'lucide-react'
import { useToast } from '@/components/ui/use-toast'
import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastProvider,
  ToastTitle,
  ToastViewport,
} from '@/components/ui/toast'

export function Toaster() {
  const { toasts } = useToast()

  return (
    <ToastProvider swipeDirection="right" duration={4000}>
      {toasts.map(({ id, title, description, action, variant = 'default', ...props }) => (
        <Toast key={id} variant={variant} {...props}>
          <div className="flex items-start gap-3">
            <span className="mt-0.5 shrink-0">
              {variant === 'success' && <CircleCheck className="h-4 w-4" />}
              {variant === 'destructive' && <TriangleAlert className="h-4 w-4" />}
              {variant === 'default' && <Info className="h-4 w-4" />}
            </span>
            <div className="grid gap-0.5">
              {title && <ToastTitle>{title}</ToastTitle>}
              {description && <ToastDescription>{description}</ToastDescription>}
            </div>
            {action}
          </div>
          <ToastClose />
        </Toast>
      ))}
      <ToastViewport />
    </ToastProvider>
  )
}
