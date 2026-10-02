import { useState } from 'react'
import { bulkDeleteUserManagerAccounts } from '@/api/user-manager.ts'
import { useDeleteUserManagerAccountMutation } from '@/hooks/user-manager/useDeleteUserManagerAccountMutation.ts'
import { useUserManagerAccountsListQuery } from '@/hooks/user-manager/useUserManagerAccountsListQuery.ts'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { AccountForm } from '@/features/user-manager/components/account-form/account-form.tsx'
import { AccountConfigDialog } from '@/features/user-manager/components/dialogs/account-config-dialog.tsx'
import { AccountPasswordDialog } from '@/features/user-manager/components/dialogs/account-password-dialog.tsx'
import { AccountShareDialog } from '@/features/user-manager/components/dialogs/account-share-dialog.tsx'
import { BulkCreateDialog } from '@/features/user-manager/components/dialogs/bulk-create-dialog.tsx'
import { BulkImportDialog } from '@/features/user-manager/components/dialogs/bulk-import-dialog.tsx'
import { useUserManager } from '@/features/user-manager/context/user-manager-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { ExpiredEntitiesDialog } from '@/features/shared-components/table/dialogs/expired-entities-dialog.tsx'

interface Props {
  accountsList: UserManagerAccount[]
}

export function AccountDialogs({ accountsList }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useUserManager()
  const { mutateAsync } = useDeleteUserManagerAccountMutation()
  const { refetch: refetchAccountsList } = useUserManagerAccountsListQuery()
  const [pending, setPending] = useState(false)

  const expiredRows = accountsList
    .filter((a) => a.status.includes('expired') || a.status.includes('suspended'))
    .map((a) => ({
      id: a.id,
      label: a.username,
      reasons: [
        ...(a.status.includes('expired') ? ['منقضی‌شده'] : []),
        ...(a.status.includes('suspended') ? ['سهمیه تمام‌شده'] : []),
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
      <BulkImportDialog
        open={open === 'bulk-import'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulk-import' : null)}
      />

      <BulkCreateDialog
        open={open === 'bulk-create'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulk-create' : null)}
      />

      <ExpiredEntitiesDialog
        open={open === 'expired'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'expired' : null)}
        entityLabel='حساب'
        rows={expiredRows}
        isLoading={false}
        onDelete={async (ids) => {
          const result = await bulkDeleteUserManagerAccounts(ids)
          await refetchAccountsList()
          return result
        }}
      />

      <Dialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
      >
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader className='text-left'>
            <DialogTitle>ایجاد حساب جدید</DialogTitle>
            <DialogDescription>
              فرم زیر را برای ایجاد یک حساب جدید L2TP/PPTP/SSTP/OpenVPN پر
              کنید.
            </DialogDescription>
          </DialogHeader>
          <AccountForm
            onClose={() => setOpen(null)}
            setIsLoading={setPending}
          />
          <DialogFooter>
            <Button
              disabled={pending}
              type='submit'
              form='user-manager-account-form'
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
                  حساب را در زیر به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <AccountForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='user-manager-account-form'
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
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />

          <AccountShareDialog
            key={`account-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <AccountConfigDialog
            key={`account-config-${currentRow.id}`}
            open={open === 'config'}
            onOpenChange={handleClose('config')}
            currentRow={currentRow}
          />

          <AccountPasswordDialog
            key={`account-password-${currentRow.id}`}
            open={open === 'password'}
            onOpenChange={handleClose('password')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
