import { useDeleteInterfaceMutation } from '@/hooks/interfaces/useDeleteInterfaceMutation.ts'
import { useInterfaces } from '@/features/interfaces/context/interfaces-context.tsx'
import { InterfacesSyncDialog } from '@/features/interfaces/components/interfaces-sync-dialog.tsx'
import { ActionDialog } from '@/features/shared-components/table/dialogs/action-dialog.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'

export function InterfacesDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useInterfaces()
  const { mutateAsync } = useDeleteInterfaceMutation()

  const handleClose = (type: typeof open) => () => {
    setOpen(type)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <InterfacesSyncDialog />
      <ActionDialog
        open={open === 'add'}
        onOpenChange={(isOpen) => setOpen(isOpen ? 'add' : null)}
        title='افزودن اینترفیس جدید'
        description='فرم زیر را برای ایجاد اینترفیس جدید تکمیل کنید.'
        formId='interface-form'
      />

      {currentRow && (
        <>
          <ActionDialog
            open={open === 'edit'}
            onOpenChange={handleClose('edit')}
            title='ویرایش اینترفیس'
            description='اطلاعات اینترفیس را در زیر به‌روزرسانی کنید.'
            currentRow={currentRow}
            formId='interface-form'
          />

          <DeleteEntityDialog
            open={open === 'delete'}
            onOpenChange={handleClose('delete')}
            entity={{ ...currentRow, id: String(currentRow.id) }}
            entityType='اینترفیس'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />
        </>
      )}
    </>
  )
}
