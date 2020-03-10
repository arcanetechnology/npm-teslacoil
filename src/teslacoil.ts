import axios from 'axios'
import { Direction, Invoice, PaymentStatus, TeslaError, Transaction } from './types'

const api = axios.create({
  validateStatus: () => true,
})

const apiKeyNotSetMessage = "looks like you haven't set your api-key! set api-key by calling setCredentials(key)"
type environments = 'MAINNET' | 'TESTNET'
let apiKey = ''

export const setCredentials = (key: string, network: environments = 'MAINNET'): void => {
  if (key === '') {
    throw Error('api key can not be set to empty string')
  }

  apiKey = key
  api.defaults.baseURL = network === 'MAINNET' ? 'https://api.teslacoil.io' : 'https://testnetapi.teslacoil.io'
  api.defaults.timeout = 5000
  api.defaults.headers = { Authorization: apiKey }
}

export const getByPaymentRequest = async (paymentRequest: string): Promise<Invoice> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }
  try {
    const response = await api.get(`v0/invoices?payment_request=${paymentRequest}`)
    return response.data as Invoice
  } catch (error) {
    throw Error(error)
  }
}

export const getInvoiceById = async (uuid: string): Promise<Invoice> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }
  try {
    const response = await api.get(`/v0/invoices?uuid=${uuid}`)
    return response.data as Invoice
  } catch (error) {
    throw Error(error)
  }
}

interface CreateInvoiceArgs {
  amount: number // must be greater than 0 and less than 4294968
  callback_url?: string
  client_id?: string
  currency: string
  exchange_currency?: string
  expiry_seconds?: number
  lightning_memo?: string
  description?: string
}

export const createInvoice = async (args: CreateInvoiceArgs): Promise<Invoice> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }

  try {
    const response = await api.post('v0/invoices/lightning', args)
    return response.data as Invoice
  } catch (error) {
    throw Error(error)
  }
}

interface PayInvoiceArgs {
  payment_request: string
  description?: string
}

export const payInvoiceSync = async (args: PayInvoiceArgs): Promise<Transaction | TeslaError> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }
  const syncApi = axios.create({
    validateStatus: () => true,
  })

  syncApi.defaults.baseURL = api.defaults.baseURL
  syncApi.defaults.timeout = 120000
  syncApi.defaults.headers = api.defaults.headers

  const response = await syncApi.post('/v0/transactions/lightning/send', args)
  return response.data as Transaction
}

export { Invoice, PaymentStatus, Direction }
