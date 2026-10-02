import { useState } from 'react'
import { Application } from '@/schema/application.ts'
import { Button } from '@/components/ui/button.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { useDeleteApplicationMutation } from '@/hooks/applications/useDeleteApplicationMutation.ts'
import { ApplicationCredentialsDialog } from '@/features/applications/components/dialogs/application-credentials-dialog.tsx'
import { ApplicationDetailsDialog } from '@/features/applications/components/dialogs/application-details-dialog.tsx'
import { ApplicationEditForm } from '@/features/applications/components/application-edit-form.tsx'
import { ApplicationForm } from '@/features/applications/components/application-form.tsx'
import { useApplication } from '@/features/applications/context/application-context.tsx'
import { DeleteEntityDialog } from '@/features/shared-components/table/dialogs/delete-entity-dialog.tsx'

export function ApplicationDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useApplication()
  const { mutateAsync } = useDeleteApplicationMutation()
  const [pending, setPending] = useState(false)
  const [createdApplication, setCreatedApplication] =
    useState<Application | null>(null)

  const handleCreated = (application: Application) => {
    setOpen(null)
    setCreatedApplication(application)
  }

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
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader className='text-left'>
            <DialogTitle>ساخت اپلیکیشن</DialogTitle>
            <DialogDescription>
              با انتخاب پروتکل‌ها/منابع مجاز، یک اپلیکیشن جدید بسازید.
            </DialogDescription>
          </DialogHeader>
          <ApplicationForm onCreated={handleCreated} setIsLoading={setPending} />
          <DialogFooter>
            <Button disabled={pending} type='submit' form='application-form'>
              ساخت
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {currentRow && (
        <>
          <Dialog open={open === 'edit'} onOpenChange={handleClose('edit')}>
            <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
              <DialogHeader className='text-left'>
                <DialogTitle>ویرایش اپلیکیشن</DialogTitle>
              </DialogHeader>
              <ApplicationEditForm
                currentRow={currentRow}
                onClose={() => handleClose('edit')(false)}
                setIsLoading={setPending}
              />
              <DialogFooter>
                <Button
                  disabled={pending}
                  type='submit'
                  form='application-edit-form'
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
              name: currentRow.name || currentRow.app_username,
            }}
            entityType='اپلیکیشن'
            mutationFn={async (id: string) => mutateAsync(Number(id))}
          />

          <ApplicationDetailsDialog
            key={`application-details-${currentRow.id}`}
            open={open === 'details'}
            onOpenChange={handleClose('details')}
            currentRow={currentRow}
          />
        </>
      )}

      <ApplicationCredentialsDialog
        open={createdApplication !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) setCreatedApplication(null)
        }}
        application={createdApplication}
      />
    </>
  )
}
