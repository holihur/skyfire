import * as ToastPrim from '@radix-ui/react-toast'
import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { CheckCircledIcon, Cross2Icon, InfoCircledIcon, ExclamationTriangleIcon } from '@radix-ui/react-icons'
import { clsx } from 'clsx'

type Kind = 'success' | 'error' | 'info'

interface ToastState {
  push: (message: string, kind?: Kind) => void
}

const ToastCtx = createContext<ToastState>({ push: () => {} })

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  const [msg, setMsg] = useState('')
  const [kind, setKind] = useState<Kind>('info')

  const push = useCallback((message: string, k: Kind = 'info') => {
    setMsg(message)
    setKind(k)
    setOpen(true)
  }, [])

  const value = useMemo(() => ({ push }), [push])

  return (
    <ToastCtx.Provider value={value}>
      <ToastPrim.Provider swipeDirection="right" duration={4000}>
        {children}
        <ToastPrim.Root
          open={open}
          onOpenChange={setOpen}
          className={clsx(
            'pointer-events-auto flex items-start gap-3 rounded-xl border bg-panel2 px-4 py-3 shadow-xl shadow-shade/30',
            kind === 'success' && 'border-ok/40 text-ok',
            kind === 'error' && 'border-err/40 text-err',
            kind === 'info' && 'border-info/40 text-info',
          )}
        >
          <span className="mt-0.5">
            {kind === 'success' && <CheckCircledIcon className="h-4 w-4" />}
            {kind === 'error' && <ExclamationTriangleIcon className="h-4 w-4" />}
            {kind === 'info' && <InfoCircledIcon className="h-4 w-4" />}
          </span>
          <span className="text-sm">{msg}</span>
          <ToastPrim.Close className="ml-2 opacity-60 transition hover:opacity-100">
            <Cross2Icon className="h-4 w-4" />
          </ToastPrim.Close>
        </ToastPrim.Root>
        <ToastPrim.Viewport className="fixed bottom-4 right-4 z-50 flex w-[calc(100vw-2rem)] max-w-sm flex-col gap-2 outline-none" />
      </ToastPrim.Provider>
    </ToastCtx.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast() {
  return useContext(ToastCtx)
}