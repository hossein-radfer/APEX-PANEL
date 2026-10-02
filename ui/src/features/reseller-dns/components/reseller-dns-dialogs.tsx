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
import { useDeleteDNSAccountForResellerMutation } from '@/hooks/dns-account/useDeleteDNSAccountForResellerMutation.ts'
import { DNSAccountShareDialog } from '@/features/dns-accounts/components/dialogs/dns-account-share-dialog.tsx'
import { DNSAccountForm } from '@/features/dns-accounts/components/dns-account-form.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { useResellerDNS } from '@/features/reseller-dns/context/reseller-dns-context.tsx'

interface Props {
  resellerId: number
}

export function ResellerDNSDialogs({ resellerId }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useResellerDNS()
  const { mutateAsync } = useDeleteDNSAccountForResellerMutation()
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
            <DialogTitle>ایجاد حساب جدید</DialogTitle>
            <DialogDescription>
              فرم را برای ایجاد یک حساب DNS جدید برای این نماینده تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <DNSAccountForm
            onClose={() => setOpen(null)}
            setIsLoading={setPending}
            targetResellerId={resellerId}
            formId='reseller-dns-account-form'
          />
          <DialogFooter>
            <Button
              disabled={pending}
              type='submit'
              form='reseller-dns-account-form'
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
                  حساب زیر را به‌روزرسانی کنید.
                </DialogDescription>
              </DialogHeader>
              <DNSAccountForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
                targetResellerId={resellerId}
                formId='reseller-dns-account-form'
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='reseller-dns-account-form'
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
            entityType='حساب'
            mutationFn={async (id: string) =>
              mutateAsync({ resellerId, accountId: Number(id) })
            }
          />

          <DNSAccountShareDialog
            key={`reseller-dns-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
