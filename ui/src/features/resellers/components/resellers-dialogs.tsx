import { useResellers } from '@/features/resellers/context/resellers-context.tsx'
import { ActionDialog } from '@/features/shared-components/table/dialogs/action-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'
import { useDeleteResellerMutation } from '@/hooks/resellers/useDeleteResellerMutation.ts'

export function ResellersDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useResellers()
  const { mutateAsync: deleteReseller } = useDeleteResellerMutation()

  const handleClose = (type: typeof open) => () => {
    setOpen(type)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <ActionDialog
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
        title='افزودن نماینده جدید'
        description='فرم زیر را برای ایجاد نماینده جدید تکمیل کنید.'
        formId='reseller-form'
      />

      {currentRow && (
        <>
          <ActionDialog
            open={open === 'edit'}
            onOpenChange={handleClose('edit')}
            title='ویرایش نماینده'
            description='اطلاعات نماینده را در زیر به‌روزرسانی کنید.'
            currentRow={currentRow}
            formId='reseller-form'
          />

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{ ...currentRow, id: String(currentRow.id) }}
            entityType='نماینده'
            mutationFn={async (id: string) => deleteReseller(Number(id))}
          />
        </>
      )}
    </>
  )
}
