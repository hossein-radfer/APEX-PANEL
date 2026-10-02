import { useEffect, useMemo, useState } from 'react'
import { IconTrash } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// Generic row shape this dialog needs -- Peer/UserManagerAccount/
// V2RayPackage all satisfy this after their own "is this row expired or
// quota-exhausted" mapping (see isExpiredOrExhausted usage at each call
// site), so one dialog implementation covers all three creation pages
// instead of three near-identical copies.
export interface ExpiredEntityRow {
  id: number
  label: string
  reasons: string[]
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  entityLabel: string
  rows: ExpiredEntityRow[]
  isLoading: boolean
  onDelete: (ids: number[]) => Promise<{ deleted: number[]; failedCount: number }>
}

export function ExpiredEntitiesDialog({
  open,
  onOpenChange,
  entityLabel,
  rows,
  isLoading,
  onDelete,
}: Props) {
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [isDeleting, setIsDeleting] = useState(false)

  useEffect(() => {
    if (open) setSelected(new Set(rows.map((r) => r.id)))
  }, [open, rows])

  const allSelected = useMemo(
    () => rows.length > 0 && selected.size === rows.length,
    [rows, selected]
  )

  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(rows.map((r) => r.id)))
  }

  const toggleOne = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const handleDelete = async () => {
    if (selected.size === 0) return
    setIsDeleting(true)
    try {
      const { deleted, failedCount } = await onDelete(Array.from(selected))
      if (deleted.length > 0) {
        toast.success(`${deleted.length} ${entityLabel}(s) deleted successfully`)
      }
      if (failedCount > 0) {
        toast.error(`${failedCount} ${entityLabel}(s) could not be deleted`)
      }
      if (failedCount === 0) onOpenChange(false)
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>Expired / Quota-Exhausted {entityLabel}s</DialogTitle>
          <DialogDescription>
            {entityLabel}s that have passed their expiry date or exceeded
            their traffic limit. Select which ones to permanently delete.
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className='flex items-center justify-center py-10'>
            <Loader2Icon className='h-5 w-5 animate-spin' />
          </div>
        ) : rows.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            No expired or quota-exhausted {entityLabel.toLowerCase()}s found.
          </p>
        ) : (
          <ScrollArea className='h-[min(50vh,360px)] rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-10'>
                    <Checkbox
                      checked={allSelected}
                      onCheckedChange={toggleAll}
                      aria-label='Select all'
                    />
                  </TableHead>
                  <TableHead>{entityLabel}</TableHead>
                  <TableHead>Reason</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell>
                      <Checkbox
                        checked={selected.has(row.id)}
                        onCheckedChange={() => toggleOne(row.id)}
                        aria-label={`Select ${row.label}`}
                      />
                    </TableCell>
                    <TableCell className='font-medium'>{row.label}</TableCell>
                    <TableCell>
                      <div className='flex flex-wrap gap-1'>
                        {row.reasons.map((reason) => (
                          <Badge key={reason} variant='secondary'>
                            {reason}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </ScrollArea>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant='destructive'
            disabled={selected.size === 0 || isDeleting || isLoading}
            onClick={handleDelete}
            className='gap-2'
          >
            {isDeleting ? (
              <Loader2Icon className='h-4 w-4 animate-spin' />
            ) : (
              <IconTrash className='h-4 w-4' />
            )}
            Delete Selected ({selected.size})
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
