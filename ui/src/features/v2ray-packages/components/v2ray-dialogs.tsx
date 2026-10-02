import { useState } from 'react'
import { bulkDeleteV2RayPackages } from '@/api/v2ray.ts'
import { useDeleteV2RayPackageMutation } from '@/hooks/v2ray/useDeleteV2RayPackageMutation.ts'
import { useV2RayPackagesListQuery } from '@/hooks/v2ray/useV2RayPackagesListQuery.ts'
import { V2RayPackage } from '@/schema/v2ray.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { V2RayBulkCreateDialog } from '@/features/v2ray-packages/components/dialogs/v2ray-bulk-create-dialog.tsx'
import { V2RayLiveUsageDialog } from '@/features/v2ray-packages/components/dialogs/v2ray-live-usage-dialog.tsx'
import { V2RayShareDialog } from '@/features/v2ray-packages/components/dialogs/v2ray-share-dialog.tsx'
import { V2RayForm } from '@/features/v2ray-packages/components/v2ray-form.tsx'
import { useV2Ray } from '@/features/v2ray-packages/context/v2ray-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { ExpiredEntitiesDialog } from '@/features/shared-components/table/dialogs/expired-entities-dialog.tsx'

interface Props {
  packagesList: V2RayPackage[]
}

export function V2RayDialogs({ packagesList }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useV2Ray()
  const { mutateAsync } = useDeleteV2RayPackageMutation()
  const { refetch: refetchPackagesList } = useV2RayPackagesListQuery()
  const [pending, setPending] = useState(false)

  const expiredRows = packagesList
    .filter(
      (p) =>
        p.status === 'expired' ||
        p.status === 'suspended' ||
        p.used_bytes >= p.total_volume_bytes
    )
    .map((p) => ({
      id: p.id,
      label: p.customer_label || p.uuid,
      reasons: [
        ...(p.status === 'expired' ? ['منقضی‌شده'] : []),
        ...(p.status === 'suspended' ? ['معلق'] : []),
        ...(p.used_bytes >= p.total_volume_bytes ? ['حجم تمام‌شده'] : []),
      ],
    }))

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
            <DialogTitle>ایجاد بسته جدید</DialogTitle>
            <DialogDescription>
              فرم زیر را برای ایجاد یک بسته‌ی V2Ray جدید تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <V2RayForm onClose={() => setOpen(null)} setIsLoading={setPending} />
          <DialogFooter>
            <Button
              disabled={pending}
              type='submit'
              form='v2ray-package-form'
            >
              ذخیره‌ی تغییرات
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <V2RayBulkCreateDialog
        open={open === 'bulk-create'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulk-create' : null)}
      />

      <ExpiredEntitiesDialog
        open={open === 'expired'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'expired' : null)}
        entityLabel='بسته'
        rows={expiredRows}
        isLoading={false}
        onDelete={async (ids) => {
          const result = await bulkDeleteV2RayPackages(ids)
          await refetchPackagesList()
          return result
        }}
      />

      {currentRow && (
        <>
          <Dialog open={open === 'edit'} onOpenChange={handleClose('edit')}>
            <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
              <DialogHeader className='text-left'>
                <DialogTitle>ویرایش بسته</DialogTitle>
                <DialogDescription>
                  بسته را در زیر به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <V2RayForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='v2ray-package-form'
                >
                  ذخیره‌ی تغییرات
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
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />

          <V2RayShareDialog
            key={`v2ray-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <V2RayLiveUsageDialog
            key={`v2ray-live-usage-${currentRow.id}`}
            open={open === 'live-usage'}
            onOpenChange={handleClose('live-usage')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
