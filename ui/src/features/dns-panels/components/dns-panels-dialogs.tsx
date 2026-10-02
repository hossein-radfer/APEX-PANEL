import { useState } from 'react'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { useDeleteDNSPanelMutation } from '@/hooks/dns-panel/useDeleteDNSPanelMutation.ts'
import { DNSPanelForm } from '@/features/dns-panels/components/dns-panel-form.tsx'
import { useDNSPanels } from '@/features/dns-panels/context/dns-panels-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'

export function DNSPanelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useDNSPanels()
  const { mutateAsync } = useDeleteDNSPanelMutation()
  const [pending, setPending] = useState(false)

  const handleClose = (type: typeof open) => (isOpen: boolean) => {
    setOpen(isOpen ? type : null)
    if (!isOpen) {
      setTimeout(() => setCurrentRow(null), 500)
    }
  }

  return (
    <>
      <Dialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      >
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader className='text-left'>
            <DialogTitle>افزودن پنل DNS</DialogTitle>
            <DialogDescription>
              یک پنل doctor-dns جدید ثبت کنید که حساب‌های DNS روی آن ایجاد
              می‌شوند.
            </DialogDescription>
          </DialogHeader>
          <DNSPanelForm onClose={() => setOpen(null)} setIsLoading={setPending} />
          <DialogFooter>
            <Button disabled={pending} type='submit' form='dns-panel-form'>
              ذخیره تغییرات
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {currentRow && (
        <>
          <Dialog open={open === 'edit'} onOpenChange={handleClose('edit')}>
            <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
              <DialogHeader className='text-left'>
                <DialogTitle>ویرایش پنل DNS</DialogTitle>
                <DialogDescription>
                  پنل زیر را به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <DNSPanelForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='dns-panel-form'
                >
                  ذخیره تغییرات
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{ ...currentRow, id: String(currentRow.id) }}
            entityType='پنل'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />
        </>
      )}
    </>
  )
}
