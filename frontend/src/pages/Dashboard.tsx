import { useEffect, useState } from 'react'
import { PlusIcon } from '@radix-ui/react-icons'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import { useI18n } from '../i18n'
import InterfaceCard from '../components/InterfaceCard'
import InterfaceFormDialog from '../components/InterfaceFormDialog'
import type { WireGuardInterface } from '../lib/types'

export default function Dashboard() {
  const { push } = useToast()
  const { t } = useI18n()
  const [items, setItems] = useState<WireGuardInterface[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [edit, setEdit] = useState<WireGuardInterface | null>(null)

  const load = async () => {
    try {
      setItems(await api.interfaces())
    } catch (err) {
      push(String(err), 'error')
    }
  }

  useEffect(() => {
    void load()
    const id = setInterval(load, 4000)
    return () => clearInterval(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="mx-auto max-w-6xl">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-fg">{t('dashboard.title')}</h1>
          <p className="mt-0.5 text-sm text-muted">{t('dashboard.subtitle')}</p>
        </div>
        <button className="btn-primary" onClick={() => setCreateOpen(true)}>
          <PlusIcon className="h-4 w-4" /> {t('dashboard.new')}
        </button>
      </div>

      {items.length === 0 ? (
        <div className="card flex flex-col items-center justify-center gap-4 py-20 text-center">
          <span className="text-4xl">🛜</span>
          <p className="text-muted">{t('dashboard.empty')}</p>
          <button className="btn-primary" onClick={() => setCreateOpen(true)}>
            <PlusIcon className="h-4 w-4" /> {t('dashboard.createFirst')}
          </button>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {items.map((iface) => (
            <InterfaceCard
              key={iface.name}
              iface={iface}
              onDeleted={(name) => setItems((prev) => prev.filter((i) => i.name !== name))}
              onEdited={(updated) =>
                setItems((prev) => prev.map((i) => (i.name === updated.name ? updated : i)))
              }
            />
          ))}
        </div>
      )}

      {createOpen && (
        <InterfaceFormDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          onDone={() => void load()}
        />
      )}
      {edit && (
        <InterfaceFormDialog
          open={!!edit}
          onOpenChange={(v) => !v && setEdit(null)}
          existing={edit}
          onDone={() => {
            setEdit(null)
            void load()
          }}
        />
      )}
    </div>
  )
}