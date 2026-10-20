import { h } from 'vue'
import { format } from 'date-fns'
import dropdown from './editorPromptDropdown.vue'

export const createEditorPromptColumns = (t, { onEdit } = {}) => [
  {
    accessorKey: 'title',
    header: () => h('div', { class: 'text-left' }, t('globals.terms.title')),
    cell: ({ row }) =>
      h(
        'div',
        { class: 'text-left' },
        onEdit
          ? h(
              'span',
              {
                class: 'text-foreground font-medium hover:underline cursor-pointer',
                onClick: () => onEdit(row.original)
              },
              row.getValue('title')
            )
          : row.getValue('title')
      )
  },
  {
    accessorKey: 'updated_at',
    enableGlobalFilter: false,
    header: () => h('div', { class: 'text-left' }, t('globals.terms.updatedAt')),
    cell: ({ row }) =>
      h('div', { class: 'text-left' }, format(row.getValue('updated_at'), 'PPpp'))
  },
  {
    id: 'actions',
    enableHiding: false,
    enableSorting: false,
    cell: ({ row }) => h('div', { class: 'relative' }, h(dropdown, { prompt: row.original }))
  }
]
