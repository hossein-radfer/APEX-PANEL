import { useDeleteIPPoolMutation } from '@/hooks/ip-pool/useDeleteIPPoolMutation.ts'
import { usePools } from '@/features/ip-pools/context/pools-context.tsx'
import { ActionDialog } from '@/features/shared-components/table/dialogs/action-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'

export function PoolsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = usePools()
  const { mutateAsync } = useDeleteIPPoolMutation()

  const handleClose = (type: typeof open) => () => {
    setOpen(type)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <ActionDialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
        title='ایجاد استخر جدید'
        description='فرم زیر را برای ایجاد استخر جدید تکمیل کنید.'
        formId='pool-form'
      />

      {currentRow && (
        <>
          <ActionDialog
            open={open === 'edit'}
            onOpenChange={handleClose('edit')}
            title='ویرایش استخر'
            description='اطلاعات استخر را در زیر به‌روزرسانی کنید.'
            currentRow={currentRow}
            formId='pool-form'
          />

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{ ...currentRow, id: String(currentRow.id) }}
            entityType='استخر'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />
        </>
      )}
    </>
  )
}
