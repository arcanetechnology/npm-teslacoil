import axios from 'axios'
import { Direction, Invoice, Status, TeslaError } from './types'

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
    const response = await api.get(`/invoices/${paymentRequest}`)
    return response.data
  } catch (error) {
    throw Error(error)
  }
}

interface CreateInvoiceArgs {
  amountSat: number // must be greater than 0 and less than 4294968
  memo?: string
  description?: string
  callbackUrl?: string
  orderId?: string
}

export const createInvoice = async (args: CreateInvoiceArgs): Promise<Invoice> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }

  try {
    const response = await api.post('/invoices/create', args)
    return response.data as Invoice
  } catch (error) {
    throw Error(error)
  }
}

interface PayInvoiceArgs {
  paymentRequest: string
  description?: string
}

export const payInvoice = async (args: PayInvoiceArgs): Promise<Invoice | TeslaError> => {
  if (apiKey === '') {
    throw Error(apiKeyNotSetMessage)
  }
  const response = await api.post('/invoices/pay', args)
  return response.data
}

export { Invoice, Status, Direction }
