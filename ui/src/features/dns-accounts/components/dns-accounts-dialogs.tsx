import { useState } from 'react'
import { bulkDeleteDNSAccounts } from '@/api/dns-account.ts'
import { useDeleteDNSAccountMutation } from '@/hooks/dns-account/useDeleteDNSAccountMutation.ts'
import { useDNSAccountsListQuery } from '@/hooks/dns-account/useDNSAccountsListQuery.ts'
import { DNSAccount } from '@/schema/dns-account.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { DNSAccountShareDialog } from '@/features/dns-accounts/components/dialogs/dns-account-share-dialog.tsx'
import { DNSAccountForm } from '@/features/dns-accounts/components/dns-account-form.tsx'
import { useDNSAccounts } from '@/features/dns-accounts/context/dns-accounts-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { ExpiredEntitiesDialog } from '@/features/shared-components/table/dialogs/expired-entities-dialog.tsx'

interface Props {
  accountsList: DNSAccount[]
}

export function DNSAccountsDialogs({ accountsList }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useDNSAccounts()
  const { mutateAsync } = useDeleteDNSAccountMutation()
  const { refetch: refetchAccountsList } = useDNSAccountsListQuery()
  const [pending, setPending] = useState(false)

  const expiredRows = accountsList
    .filter(
      (a) =>
        a.status === 'expired' ||
        a.status === 'suspended' ||
        (a.total_volume_bytes > 0 && a.used_bytes >= a.total_volume_bytes)
    )
    .map((a) => ({
      id: a.id,
      label: a.customer_label || a.uuid,
      reasons: [
        ...(a.status === 'expired' ? ['منقضی‌شده'] : []),
        ...(a.status === 'suspended' ? ['معلق'] : []),
        ...(a.total_volume_bytes > 0 && a.used_bytes >= a.total_volume_bytes
          ? ['حجم تمام‌شده']
          : []),
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
            <DialogTitle>ایجاد حساب جدید</DialogTitle>
            <DialogDescription>
              فرم زیر را برای ایجاد یک حساب Smart DNS جدید تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <DNSAccountForm onClose={() => setOpen(null)} setIsLoading={setPending} />
          <DialogFooter>
            <Button disabled={pending} type='submit' form='dns-account-form'>
              ذخیره‌ی تغییرات
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ExpiredEntitiesDialog
        open={open === 'expired'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'expired' : null)}
        entityLabel='حساب'
        rows={expiredRows}
        isLoading={false}
        onDelete={async (ids) => {
          const result = await bulkDeleteDNSAccounts(ids)
          await refetchAccountsList()
          return result
        }}
      />

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
              <DNSAccountForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='dns-account-form'
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
            entityType='حساب'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />

          <DNSAccountShareDialog
            key={`dns-account-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
