import { bulkDeletePeers } from '@/api/peers.ts'
import { useDeletePeerMutation } from '@/hooks/peers/useDeletePeerMutation.ts'
import { usePeersListQuery } from '@/hooks/peers/usePeersListQuery.ts'
import { Peer } from '@/schema/peers.ts'
import { PeersBulkCreateDialog } from '@/features/peers/components/dialogs/peers-bulk-create-dialog.tsx'
import { PeersConfigDialog } from '@/features/peers/components/dialogs/peers-config-dialog.tsx'
import { PeersQRCodeDialog } from '@/features/peers/components/dialogs/peers-qrcode-dialog.tsx'
import { PeersSyncDialog } from '@/features/peers/components/dialogs/peers-sync-dialog.tsx'
import { PeersShareDialog } from '@/features/peers/components/dialogs/peers-share-dialog.tsx'
import { usePeers } from '@/features/peers/context/peers-context.tsx'
import { ActionDialog } from '@/features/shared-components/table/dialogs/action-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { ExpiredEntitiesDialog } from '@/features/shared-components/table/dialogs/expired-entities-dialog.tsx'

interface Props {
  peersList: Peer[]
}

export function PeersDialogs({ peersList }: Props) {
  const { open, setOpen, currentRow, setCurrentRow } = usePeers()
  const { mutateAsync } = useDeletePeerMutation()
  const { refetch: refetchPeersList } = usePeersListQuery()

  const expiredRows = peersList
    .filter((p) => p.status.includes('expired') || p.status.includes('suspended'))
    .map((p) => ({
      id: p.id,
      label: p.name,
      reasons: [
        ...(p.status.includes('expired') ? ['منقضی‌شده'] : []),
        ...(p.status.includes('suspended') ? ['اتمام حجم'] : []),
      ],
    }))

  const handleClose = (type: typeof open) => () => {
    setOpen(type)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <PeersSyncDialog />
      <PeersBulkCreateDialog
        open={open === 'bulk-create'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'bulk-create' : null)}
      />
      <ExpiredEntitiesDialog
        open={open === 'expired'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'expired' : null)}
        entityLabel='وایرگارد'
        rows={expiredRows}
        isLoading={false}
        onDelete={async (ids) => {
          const result = await bulkDeletePeers(ids)
          await refetchPeersList()
          return result
        }}
      />
      <ActionDialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
        title='ایجاد وایرگارد جدید'
        description='فرم زیر را برای ساخت وایرگارد جدید تکمیل کنید.'
        formId='peer-form'
      />

      {currentRow && (
        <>
          <ActionDialog
            open={open === 'edit'}
            onOpenChange={handleClose('edit')}
            title='ویرایش وایرگارد'
            description='اطلاعات وایرگارد زیر را به‌روزرسانی کنید.'
            currentRow={currentRow}
            formId='peer-form'
          />

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{ ...currentRow, id: String(currentRow.id) }}
            entityType='وایرگارد'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />

          <PeersShareDialog
            key={`peer-share-${currentRow.id}`}
            open={open === 'share'}
            onOpenChange={handleClose('share')}
            currentRow={currentRow}
          />

          <PeersQRCodeDialog
            key={`peer-qrcode-${currentRow.id}`}
            open={open === 'qrCode'}
            onOpenChange={handleClose('qrCode')}
            currentRow={currentRow}
          />

          <PeersConfigDialog
            key={`peer-config-${currentRow.id}`}
            open={open === 'show_config' || open === 'download_config'}
            download={open === 'download_config'}
            onOpenChange={open === 'download_config' ? handleClose('download_config') : handleClose('show_config')}
            currentRow={currentRow}
          />
        </>
      )}
    </>
  )
}
