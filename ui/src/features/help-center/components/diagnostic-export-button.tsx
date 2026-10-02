import { Stethoscope } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useDiagnosticExport } from '@/hooks/help-center/useDiagnosticExport.ts'

interface DiagnosticExportButtonProps {
  onExported: (file: File) => void
}

export function DiagnosticExportButton({ onExported }: DiagnosticExportButtonProps) {
  const { runExport, isExporting } = useDiagnosticExport(onExported)

  return (
    <Button
      type='button'
      variant='outline'
      size='icon'
      onClick={runExport}
      disabled={isExporting}
      title='ارسال گزارش تشخیصی (اطلاعات سرور، وضعیت لایسنس، لاگ‌های اخیر)'
    >
      <Stethoscope className='size-4' />
    </Button>
  )
}
