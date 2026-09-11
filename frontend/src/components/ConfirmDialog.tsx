import Dialog from './Dialog'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  title: string
  children: React.ReactNode
  onConfirm: () => void
  confirmLabel?: string
  danger?: boolean
}

export default function ConfirmDialog({
  open,
  onOpenChange,
  title,
  children,
  onConfirm,
  confirmLabel = 'Confirm',
  danger,
}: Props) {
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      footer={
        <>
          <button
            className={danger ? 'btn-danger px-4 py-2' : 'btn-primary'}
            autoFocus
            onClick={() => {
              onOpenChange(false)
              onConfirm()
            }}
          >
            {confirmLabel}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </button>
        </>
      }
    >
      {children}
    </Dialog>
  )
}