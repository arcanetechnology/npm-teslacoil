export type Direction = 'INBOUND' | 'OUTBOUND'
export type PaymentStatus = 'PAID' | 'UNPAID' | 'OVERPAID' | 'UNDERPAID'

export interface Invoice {
  uuid: string
  user_uuid: string
  transaction_uuids: Array<string>
  requested_amount_satoshi: number
  requested_amount_bitcoin: number
  expiry_seconds: number
  payment_status: PaymentStatus
  paid_before_expiry: boolean
  settle_time?: Date
  callback_url: string
  description: string
  client_id: string
  type: string
  create_time: Date
  exchange_currency: string
  fiat_currency: string
  requested_amount_fiat: number
  bitcoin_address: string
  lightning_payment_request: string
}

export interface Transaction {
  uuid: string
  user_uuid: string
  invoice_uuid: string
  callback_url: string
  client_id: string
  direction: Direction
  amount_satoshi: number
  amount_bitcoin: number
  network_fee_satoshi: number
  network_fee_bitcoin: number
  description: string
  type: string
  status: string
  create_time: Date
}

export interface TeslaError {
  error: string
  docs: string
}
