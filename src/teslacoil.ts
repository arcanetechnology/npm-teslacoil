import axios from "axios";
const api = axios.create();

const apiKeyNotSetMessage = "looks like you haven't set your api-key! set api-key by calling setCredentials(key)";
type environments = "MAINNET" | "TESTNET";
let apiKey = "";

export const setCredentials = (key = "", network: environments = "MAINNET") => {
  apiKey = key;
  api.defaults.baseURL = network === "MAINNET" ? "https://api.teslacoil.io" : "https://testnetapi.teslacoil.io";
  api.defaults.timeout = 5000;
  api.defaults.headers = { Authorization: apiKey };
};

export const getByPaymentRequest = async (paymentRequest: string) => {
  if (apiKey == "") {
    throw Error(apiKeyNotSetMessage);
  }
  try {
    const response = await api.get(`/invoices/${paymentRequest}`);
    return response.data;
  } catch (error) {
    throw Error(error);
  }
};

export const decodePaymentRequest = async (paymentRequest: string) => {
  try {
    const response = await api.get(`/invoices/${paymentRequest}`);
    return response.data;
  } catch (err) {
    throw Error(err);
  }
};

interface CreateInvoiceArgs {
  amountSat: number; // must be greater than 0 and less than 4294968
  memo?: string;
  description?: string;
  callbackUrl?: string;
  orderId?: string;
}

export const createInvoice = async (args: CreateInvoiceArgs) => {
  if (apiKey == "") {
    throw Error(apiKeyNotSetMessage);
  }

  try {
    const response = await api.post("/invoices/create", args);
    return response.data;
  } catch (error) {
    throw Error(error);
  }
};

interface PayInvoiceArgs {
  paymentRequest: string;
  description?: string;
}

const payInvoice = async (args: PayInvoiceArgs) => {
  if (apiKey == "") {
    throw Error(apiKeyNotSetMessage);
  }
  try {
    const response = await api.post("/invoices/pay", args);
    return response.data;
  } catch (error) {
    throw Error(error);
  }
};
