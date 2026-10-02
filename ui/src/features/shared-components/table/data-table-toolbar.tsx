import { Cross2Icon } from '@radix-ui/react-icons'
import { Table } from '@tanstack/react-table'
import { Button } from '@/components/ui/button.tsx'
import { Input } from '@/components/ui/input.tsx'
import { DataTableFacetedFilter } from './data-table-faceted-filter.tsx'
import { DataTableViewOptions } from './data-table-view-options.tsx'

interface DataTableToolbarProps<TData> {
  table: Table<TData>
  type:
    | 'Servers'
    | 'Interfaces'
    | 'Pools'
    | 'Peers'
    | 'User Manager Accounts'
    | 'V2Ray Packages'
}

const serverStatusOptions = [
  { label: 'در دسترس', value: 'available' },
  { label: 'در دسترس نیست', value: 'not_available' },
]

const interfaceStatusOptions = [
  { label: 'فعال', value: 'running' },
  { label: 'غیرفعال', value: 'not_running' },
]

const peerStatusOptions = [
  { label: 'فعال', value: 'active' },
  { label: 'غیرفعال', value: 'inactive' },
  { label: 'منقضی‌شده', value: 'expired' },
  { label: 'معلق', value: 'suspended' },
]

const v2rayPackageStatusOptions = [
  { label: 'فعال', value: 'active' },
  { label: 'معلق', value: 'suspended' },
  { label: 'منقضی‌شده', value: 'expired' },
]

// typeLabelsFa maps the internal `type` union (unchanged -- it's an
// identifier consumed by every caller of this component, not display
// copy) to the Persian label actually shown in the filter placeholder.
const typeLabelsFa: Record<DataTableToolbarProps<unknown>['type'], string> = {
  Servers: 'سرورها',
  Interfaces: 'اینترفیس‌ها',
  Pools: 'استخرها',
  Peers: 'وایرگارد',
  'User Manager Accounts': 'حساب‌های User Manager',
  'V2Ray Packages': 'پکیج‌های V2Ray',
}

function facetOptionsFrom<TData>(
  table: Table<TData>,
  columnId: string
): { label: string; value: string }[] {
  const column = table.getColumn(columnId)
  const facets = column?.getFacetedUniqueValues()
  if (!facets) return []
  return Array.from(facets.keys())
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ label: value, value }))
}

export function DataTableToolbar<TData>({
  table,
  type,
}: DataTableToolbarProps<TData>) {
  const isFiltered = table.getState().columnFilters.length > 0
  const searchColumnId =
    type === 'User Manager Accounts'
      ? 'username'
      : type === 'V2Ray Packages'
        ? 'customer_label'
        : 'name'

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 flex-col-reverse items-start gap-y-2 sm:flex-row sm:items-center sm:space-x-2'>
        <Input
          placeholder={`فیلتر ${typeLabelsFa[type]}...`}
          value={
            (table.getColumn(searchColumnId)?.getFilterValue() as string) ?? ''
          }
          onChange={(event) =>
            table.getColumn(searchColumnId)?.setFilterValue(event.target.value)
          }
          className='h-8 w-[150px] lg:w-[250px]'
        />
        <div className='flex gap-x-2'>
          {type !== 'Pools' &&
            type !== 'User Manager Accounts' &&
            table.getColumn('status') && (
              <DataTableFacetedFilter
                column={table.getColumn('status')}
                title='وضعیت'
                options={
                  type === 'Servers'
                    ? serverStatusOptions
                    : type === 'Interfaces'
                      ? interfaceStatusOptions
                      : type === 'V2Ray Packages'
                        ? v2rayPackageStatusOptions
                        : peerStatusOptions
                }
              />
            )}
          {type === 'User Manager Accounts' && (
            <>
              {table.getColumn('status') && (
                <DataTableFacetedFilter
                  column={table.getColumn('status')}
                  title='وضعیت'
                  options={peerStatusOptions}
                />
              )}
              {table.getColumn('group') && (
                <DataTableFacetedFilter
                  column={table.getColumn('group')}
                  title='گروه'
                  options={facetOptionsFrom(table, 'group')}
                />
              )}
              {table.getColumn('profile') && (
                <DataTableFacetedFilter
                  column={table.getColumn('profile')}
                  title='پروفایل'
                  options={facetOptionsFrom(table, 'profile')}
                />
              )}
              {table.getColumn('protocol') && (
                <DataTableFacetedFilter
                  column={table.getColumn('protocol')}
                  title='پروتکل'
                  options={facetOptionsFrom(table, 'protocol')}
                />
              )}
            </>
          )}
        </div>
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={() => table.resetColumnFilters()}
            className='h-8 px-2 lg:px-3'
          >
            بازنشانی
            <Cross2Icon className='ml-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}
