import Dialog from './Dialog'
import { useI18n } from '../i18n'

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
  confirmLabel,
  danger,
}: Props) {
  const { t } = useI18n()
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
            {confirmLabel ?? t('common.confirm')}
          </button>
          <button className="btn-ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      {children}
    </Dialog>
  )
}