import axios from 'axios';
const api = axios.create();

type environments = "MAINNET" | "TESTNET"
let apiKey = "";


export const setCredentials = (key = '', network: environments = 'MAINNET') => {
  apiKey = key;
  api.defaults.baseURL = (network === 'MAINNET') ? 'https://api.teslacoil.io' : 'https://testnetapi.teslacoil.io';
  api.defaults.timeout = 5000;
  api.defaults.headers = { 'Authorization' : apiKey, };
}

// transactions
export const getAllTransactions = async () => {
  try {
    const response = await api.get('/transactions');
    return response.data.data;
  } catch (error) {
    throw Error(error);
  }
}

export const getTransactionByPaymentRequest = async (paymentRequest: string) => {
  try {
    const response = await api.get(`/invoices/${paymentRequest}`);
    return response.data.data;
  } catch (error) {
    throw Error(error);
  }
}

interface CreateInvoiceArgs {
  amountSat: number; // must be greater than 0 and less than 4294968
  memo: string;
  description: string;
  callbackUrl: string;
  orderId: string;
}

export const createInvoice = async (args: CreateInvoiceArgs) => {
  try {
    const response = await api.post('/invoices/create', args);
    return response.data.data;
  } catch (error) {
    throw Error(error);
  }
}

interface PayInvoiceArgs {
  paymentRequest: string
  description: string
}

const payInvoice = async (args: PayInvoiceArgs) => {
  try {
    const response = await api.post('/invoices/pay', args);
    return response.data.data;
  } catch (error) {
    throw Error(error);
  }
}

/**
 * 
 * @param charge 
async function verifySignature(charge) {
  const hash = crypto.createHmac('sha256', api_key).update(charge.id).digest('hex');
  return hash === charge.hashed_order;
}
 */