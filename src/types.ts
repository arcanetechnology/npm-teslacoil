export type Direction = 'INBOUND' | 'OUTBOUND'
export type Status = 'CREATED' | 'SENT' | 'COMPLETED' | 'FLOPPED'

export interface Invoice {
  id: number
  userId: number
  callbackUrl?: string
  // customerOrderId is an optional field where you can specify a custom
  // order ID. The only place this is used is when hitting the callback
  // URL of a transaction.
  customerOrderId?: string

  // fields that always exist on a Invoice
  paymentRequest: string
  expiry: number
  amountSat: number
  amountMilliSat: number
  direction: Direction
  status: Status
  createdAt: Date

  // fields that sometimes exist
  description?: string
  rHash?: ArrayBuffer
  hash?: string
  rPreimage?: ArrayBuffer
  preimage?: string
  memo?: string
  settledAt?: Date
}
