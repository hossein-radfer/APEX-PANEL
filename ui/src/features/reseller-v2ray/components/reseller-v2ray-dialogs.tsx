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
import { useDeleteV2RayPackageForResellerMutation } from '@/hooks/v2ray/useDeleteV2RayPackageForResellerMutation.ts'
import { V2RayLiveUsageDialog } from '@/features/v2ray-packages/components/dialogs/v2ray-live-usage-dialog.tsx'
import { V2RayShareDialog } from '@/features/v2ray-packages/components/dialogs/v2ray-share-dialog.tsx'
import { V2RayForm } from '@/features/v2ray-packages/components/v2ray-form.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { useResellerV2Ray } from '@/features/reseller-v2ray/context/reseller-v2ray-context.tsx'

interface Props {
  resellerId: number
}

export function ResellerV2RayDialogs({ resellerId }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useResellerV2Ray()
  const { mutateAsync } = useDeleteV2RayPackageForResellerMutation()
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
            <DialogTitle>ایجاد پکیج جدید</DialogTitle>
            <DialogDescription>
              فرم را برای ایجاد یک پکیج جدید برای این نماینده تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <V2RayForm
            onClose={() => setOpen(null)}
            setIsLoading={setPending}
            targetResellerId={resellerId}
            formId='reseller-v2ray-package-form'
          />
          <DialogFooter>
            <Button
              disabled={pending}
              type='submit'
              form='reseller-v2ray-package-form'
            >
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
                <DialogTitle>ویرایش پکیج</DialogTitle>
                <DialogDescription>
                  پکیج زیر را به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <V2RayForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
                targetResellerId={resellerId}
                formId='reseller-v2ray-package-form'
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='reseller-v2ray-package-form'
                >
                  ذخیره تغییرات
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{
              ...currentRow,
              id: String(currentRow.id),
              name: currentRow.customer_label || currentRow.uuid,
            }}
            entityType='پکیج'
            mutationFn={async (id: string) =>
              mutateAsync({ resellerId, packageId: Number(id) })
            }
          />

          <V2RayShareDialog
            key={`reseller-v2ray-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <V2RayLiveUsageDialog
            key={`reseller-v2ray-live-usage-${currentRow.id}`}
            open={open === 'live-usage'}
            onOpenChange={handleClose('live-usage')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
