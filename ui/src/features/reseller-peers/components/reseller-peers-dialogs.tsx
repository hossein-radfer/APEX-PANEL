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
import { useDeletePeerForResellerMutation } from '@/hooks/peers/useDeletePeerForResellerMutation.ts'
import { PeerForm } from '@/features/peers/components/peer-form/peer-form.tsx'
import { PeersConfigDialog } from '@/features/peers/components/dialogs/peers-config-dialog.tsx'
import { PeersQRCodeDialog } from '@/features/peers/components/dialogs/peers-qrcode-dialog.tsx'
import { PeersShareDialog } from '@/features/peers/components/dialogs/peers-share-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { useResellerPeers } from '@/features/reseller-peers/context/reseller-peers-context.tsx'

interface Props {
  resellerId: number
}

export function ResellerPeersDialogs({ resellerId }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = useResellerPeers()
  const { mutateAsync } = useDeletePeerForResellerMutation()
  const [pending, setPending] = useState(false)

  // Returns an onOpenChange handler for a dialog of the given type: closing
  // it (isOpen === false, via X / overlay-click / Escape, or a successful
  // action) clears the open state and, after the close animation, clears
  // currentRow too. Re-opening it while it's already the active dialog is a
  // no-op; Radix only calls this with true when the dialog is being opened
  // via its own trigger, which these dialogs don't expose (they're opened
  // externally via row actions), so in practice isOpen is always false here.
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
            <DialogTitle>ایجاد وایرگارد جدید</DialogTitle>
            <DialogDescription>
              فرم زیر را برای ساخت وایرگارد جدید برای این نماینده تکمیل کنید.
            </DialogDescription>
          </DialogHeader>
          <PeerForm
            onClose={() => setOpen(null)}
            setIsLoading={setPending}
            targetResellerId={resellerId}
            formId='reseller-peer-form'
          />
          <DialogFooter>
            <Button disabled={pending} type='submit' form='reseller-peer-form'>
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
                <DialogTitle>ویرایش وایرگارد</DialogTitle>
                <DialogDescription>اطلاعات وایرگارد زیر را به‌روزرسانی کنید.</DialogDescription>
              </DialogHeader>
              <PeerForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
                targetResellerId={resellerId}
                formId='reseller-peer-form'
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='reseller-peer-form'
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
            entityType='وایرگارد'
            mutationFn={async (id: string) =>
              mutateAsync({ resellerId, peerId: Number(id) })
            }
          />

          <PeersShareDialog
            key={`reseller-peer-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <PeersQRCodeDialog
            key={`reseller-peer-qrcode-${currentRow.id}`}
            open={open === 'qrCode'}
            onOpenChange={handleClose('qrCode')}
            currentRow={currentRow}
          />

          <PeersConfigDialog
            key={`reseller-peer-config-${currentRow.id}`}
            open={open === 'show_config' || open === 'download_config'}
            download={open === 'download_config'}
            onOpenChange={
              open === 'download_config'
                ? handleClose('download_config')
                : handleClose('show_config')
            }
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
