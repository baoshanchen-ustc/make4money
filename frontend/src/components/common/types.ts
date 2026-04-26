/**
 * Common component types
 */

export interface Column {
  key: string
  label: string
  sortable?: boolean
  class?: string
  width?: number | string
  minWidth?: number
  maxWidth?: number
  resizable?: boolean
  formatter?: (value: any, row: any) => string
}
