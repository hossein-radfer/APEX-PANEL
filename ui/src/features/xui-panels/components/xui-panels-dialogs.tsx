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
import { useDeleteXuiPanelMutation } from '@/hooks/xui-panel/useDeleteXuiPanelMutation.ts'
import { XuiPanelForm } from '@/features/xui-panels/components/xui-panel-form.tsx'
import { useXuiPanels } from '@/features/xui-panels/context/xui-panels-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'

export function XuiPanelsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useXuiPanels()
  const { mutateAsync } = useDeleteXuiPanelMutation()
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
            <DialogTitle>افزودن پنل X-UI</DialogTitle>
            <DialogDescription>
              یک پنل x-ui جدید ثبت کنید که بسته‌های V2Ray روی آن راه‌اندازی
              می‌شوند.
            </DialogDescription>
          </DialogHeader>
          <XuiPanelForm onClose={() => setOpen(null)} setIsLoading={setPending} />
          <DialogFooter>
            <Button disabled={pending} type='submit' form='xui-panel-form'>
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
                <DialogTitle>ویرایش پنل X-UI</DialogTitle>
                <DialogDescription>
                  پنل زیر را به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <XuiPanelForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='xui-panel-form'
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
