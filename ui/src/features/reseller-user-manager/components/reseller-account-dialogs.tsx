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
import { useDeleteUserManagerAccountForResellerMutation } from '@/hooks/user-manager/useDeleteUserManagerAccountForResellerMutation.ts'
import { AccountForm } from '@/features/user-manager/components/account-form/account-form.tsx'
import { AccountConfigDialog } from '@/features/user-manager/components/dialogs/account-config-dialog.tsx'
import { AccountPasswordDialog } from '@/features/user-manager/components/dialogs/account-password-dialog.tsx'
import { AccountShareDialog } from '@/features/user-manager/components/dialogs/account-share-dialog.tsx'
import { BulkImportDialog } from '@/features/user-manager/components/dialogs/bulk-import-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { useResellerUserManager } from '@/features/reseller-user-manager/context/reseller-user-manager-context.tsx'

interface Props {
  resellerId: number
}

export function ResellerAccountDialogs({ resellerId }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useResellerUserManager()
  const { mutateAsync } = useDeleteUserManagerAccountForResellerMutation()
  const [pending, setPending] = useState(false)

  const handleClose = (type: typeof open) => (isOpen: boolean) => {
    setOpen(isOpen ? type : null)
    if (!isOpen) {
      setTimeout(() => setCurrentRow(null), 500)
    }
  }

  return (
    <>
      <BulkImportDialog
        open={open === 'bulk-import'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulk-import' : null)}
        targetResellerId={resellerId}
      />

      <Dialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      >
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader className='text-left'>
            <DialogTitle>ایجاد حساب جدید</DialogTitle>
            <DialogDescription>
              فرم زیر را برای ساخت حساب جدید برای این نماینده تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <AccountForm
            onClose={() => setOpen(null)}
            setIsLoading={setPending}
            targetResellerId={resellerId}
            formId='reseller-user-manager-account-form'
          />
          <DialogFooter>
            <Button
              disabled={pending}
              type='submit'
              form='reseller-user-manager-account-form'
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
                <DialogTitle>ویرایش حساب</DialogTitle>
                <DialogDescription>
                  اطلاعات حساب زیر را به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <AccountForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
                targetResellerId={resellerId}
                formId='reseller-user-manager-account-form'
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='reseller-user-manager-account-form'
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
              name: currentRow.username,
            }}
            entityType='حساب'
            mutationFn={async (id: string) =>
              mutateAsync({ resellerId, accountId: Number(id) })
            }
          />

          <AccountShareDialog
            key={`reseller-account-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <AccountConfigDialog
            key={`reseller-account-config-${currentRow.id}`}
            open={open === 'config'}
            onOpenChange={handleClose('config')}
            currentRow={currentRow}
            targetResellerId={resellerId}
          />

          <AccountPasswordDialog
            key={`reseller-account-password-${currentRow.id}`}
            open={open === 'password'}
            onOpenChange={handleClose('password')}
            currentRow={currentRow}
            targetResellerId={resellerId}
          />
        </>
      )}
    </>
  )
}
