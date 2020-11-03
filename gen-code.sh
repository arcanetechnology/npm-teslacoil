set -e
set -u
set -o pipefail

DEST_FILE=src/teslacoil.ts
node node_modules/restful-react/dist/bin/restful-react.js import --file teslacoil.stripped-openapi.json \
    --output $DEST_FILE --ts

echo -e "/* eslint-disable */

import axios from \"axios\";

const api = axios.create({
  validateStatus: () => true,
});

const apiKeyNotSetMessage = \"looks like you haven't set your api-key! set api-key by calling setCredentials(key)\";
type environments = \"MAINNET\" | \"TESTNET\" | \"REGTEST\";
let apiKey = \"\";

export const setCredentials = (key: string, network: environments = \"REGTEST\"): void => {
  if (key === \"\") {
    throw Error(\"api key can not be set to empty string\");
  }

  apiKey = key;
  switch (network) {
    case \"MAINNET\":
      api.defaults.baseURL = \"https://api.teslacoil.io\";
      break;
    case \"TESTNET\":
      api.defaults.baseURL = \"https://testnetapi.teslacoil.io\";
      break;
    case \"REGTEST\":
      api.defaults.baseURL = \"http://localhost:5000\";
      break;
  }
  api.defaults.timeout = 5000;
  api.defaults.headers = { Authorization: apiKey };
};

$(cat $DEST_FILE)" > $DEST_FILE