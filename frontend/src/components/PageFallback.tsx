import { useI18n } from '../i18n'

// PageFallback is shown while a lazily-loaded route component is fetched.
export default function PageFallback() {
  const { t } = useI18n()
  return (
    <div className="flex h-full items-center justify-center py-24">
      <span className="text-sm text-muted-foreground">{t('iface.loading')}</span>
    </div>
  )
}
